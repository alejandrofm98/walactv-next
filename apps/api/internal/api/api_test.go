package api

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type trackingReadCloser struct {
	io.Reader
	closed bool
}

func (b *trackingReadCloser) Close() error {
	b.closed = true
	return nil
}

func testAPI(client *http.Client) *API {
	return New(Config{Origin: "https://app.example", SessionSecret: []byte(strings.Repeat("s", 32)), IdentityKey: []byte(strings.Repeat("k", 32)), EncryptionKey: []byte(strings.Repeat("e", 32)), AllowedHosts: []string{"provider.example"}, HTTPClient: client})
}

func sessionRequest(method, target, userID string, body io.Reader) *http.Request {
	r := httptest.NewRequest(method, target, body)
	s := session{User: user{ID: userID, Username: "user", Role: "admin"}, CSRF: "csrf", Token: "token"}
	return r.WithContext(context.WithValue(r.Context(), sessionKey, s))
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
}

func TestParsePlaylistURLRejectsUnsafeAndNonXtreamURLs(t *testing.T) {
	valid := "https://provider.example/get.php?username=alice&password=a%2Fb%3Fsecret"
	base, user, pass, host, err := parsePlaylistURL(valid)
	if err != nil || base != "https://provider.example" || user != "alice" || pass != "a/b?secret" || host != "provider.example" {
		t.Fatalf("unexpected parse result: %q %q %q %q %v", base, user, pass, host, err)
	}
	if _, _, _, _, err := parsePlaylistURL("https://provider.example/get.php?username=a&password=b&type=m3u_plus&output=ts"); err != nil {
		t.Fatalf("rejected common Xtream playlist parameters: %v", err)
	}
	for _, raw := range []string{
		"file:///get.php?username=a&password=b",
		"https://provider.example/other?username=a&password=b",
		"https://user@provider.example/get.php?username=a&password=b",
		"https://provider.example/get.php?username=a&password=b&tracking=unexpected",
		"https://provider.example/get.php?username=a&password=b#fragment",
	} {
		if _, _, _, _, err := parsePlaylistURL(raw); err == nil {
			t.Errorf("accepted unsafe playlist URL %q", raw)
		}
	}
}

func TestPublicIPRejectsSpecialAddressRanges(t *testing.T) {
	for _, raw := range []string{"127.0.0.1", "10.10.1.4", "100.64.0.1", "169.254.169.254", "192.0.2.1", "198.18.0.1", "203.0.113.8", "::1", "::ffff:127.0.0.1", "fc00::1", "fe80::1", "2001:db8::1", "3fff::1"} {
		ip := netip.MustParseAddr(raw)
		if publicIP(ip) {
			t.Errorf("treated special IP %s as public", raw)
		}
	}
	for _, raw := range []string{"1.1.1.1", "2606:4700:4700::1111"} {
		if !publicIP(netip.MustParseAddr(raw)) {
			t.Errorf("rejected public IP %s", raw)
		}
	}
}

func TestAllowedHostDefaultsToAnyNonEmptyHostAndOptionalListRestricts(t *testing.T) {
	for _, hosts := range [][]string{nil, {}, {"", "  ", "\t"}} {
		s := newPrivateState(Config{AllowedHosts: hosts})
		for _, host := range []string{"provider.example", "sub.provider.example", "another-provider.example"} {
			if !s.allowedHost(host) {
				t.Errorf("empty host list rejected %q", host)
			}
		}
		if s.allowedHost("") {
			t.Fatal("empty hostname was accepted")
		}
	}

	s := newPrivateState(Config{AllowedHosts: []string{" provider.example. "}})
	if !s.allowedHost("PROVIDER.EXAMPLE") || !s.allowedHost("provider.example.") {
		t.Fatal("configured hostname was not normalized")
	}
	if s.allowedHost("another-provider.example") {
		t.Fatal("non-empty host list did not restrict providers")
	}
}

func TestSourceSecretUsesAuthenticatedEncryptionAndAAD(t *testing.T) {
	key := []byte(strings.Repeat("e", 32))
	secret := sourceSecret{OriginalURL: "https://provider.example/get.php?username=user&password=password", Base: "https://provider.example", Username: "user", Password: "password"}
	sealed, err := encryptSourceSecret(key, "owner-a", "source-a", secret)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte(secret.Username)) || bytes.Contains(sealed, []byte(secret.Password)) || bytes.Contains(sealed, []byte(secret.OriginalURL)) {
		t.Fatal("ciphertext contains private source fields in clear text")
	}
	opened, err := decryptSourceSecret(key, "owner-a", "source-a", sealed)
	if err != nil || opened != secret {
		t.Fatalf("source secret did not round trip: %#v %v", opened, err)
	}
	wrongKey := []byte(strings.Repeat("x", 32))
	if _, err := decryptSourceSecret(wrongKey, "owner-a", "source-a", sealed); err == nil {
		t.Fatal("wrong key decrypted source secret")
	}
	if _, err := decryptSourceSecret(key, "owner-b", "source-a", sealed); err == nil {
		t.Fatal("ciphertext was not bound to owner AAD")
	}
	if _, err := decryptSourceSecret(key, "owner-a", "source-b", sealed); err == nil {
		t.Fatal("ciphertext was not bound to source AAD")
	}
	tampered := append([]byte(nil), sealed...)
	tampered[len(tampered)-1] ^= 1
	if _, err := decryptSourceSecret(key, "owner-a", "source-a", tampered); err == nil {
		t.Fatal("tampered ciphertext decrypted")
	}
}

func TestPersistentCatalogCodecKeepsPlaybackMetadataButDropsProviderURLs(t *testing.T) {
	privateURL := "https://provider.example/movie/u/private-password/25.mp4"
	data := catalogData{
		categories: map[string][]category{"movie": {{ID: "7", Name: "Films"}}, "series": {{ID: "9", Name: "Drama"}}},
		items:      map[string][]CatalogItem{"movie:7": {{ID: "25", Title: "Film", Kind: "movie", ContainerExtension: "mp4", CategoryID: "7", URL: privateURL, ContentKey: strings.Repeat("a", 64)}}, "series:9": {{ID: "42", Title: "Show", Kind: "series", CategoryID: "9", SeriesID: "42"}}},
		episodes:   map[string][]Episode{"42": {{ID: "101", Title: "Pilot", Season: 1, Episode: 1, ContainerExtension: "mkv", URL: privateURL}}},
	}
	encoded, err := encodeCatalog(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{privateURL, "private-password", "direct_source"} {
		if strings.Contains(string(encoded), private) {
			t.Fatalf("persistent catalog leaked provider URL data %q: %s", private, encoded)
		}
	}
	for _, expected := range []string{`"categoryId":"7"`, `"id":"25"`, `"seriesId":"42"`, `"season":1`} {
		if !strings.Contains(string(encoded), expected) {
			t.Fatalf("persistent catalog dropped required metadata %q: %s", expected, encoded)
		}
	}
}

func TestProviderRedirectPoliciesKeepSchemeAndHostPinned(t *testing.T) {
	a := testAPI(nil)
	a.private.cfg.AllowedHosts = append(a.private.cfg.AllowedHosts, "cdn.example")
	first, _ := http.NewRequest(http.MethodGet, "https://provider.example/player_api.php", nil)
	makeReq := func(raw string) *http.Request { req, _ := http.NewRequest(http.MethodGet, raw, nil); return req }
	for _, target := range []string{"https://evil.example/steal", "https://provider.example:8443/port-change", "http://provider.example/downgrade", "file:///etc/passwd"} {
		if err := a.private.controlCheckRedirect(makeReq(target), []*http.Request{first}); err == nil {
			t.Errorf("control client accepted redirect to %s", target)
		}
	}
	open := newPrivateState(Config{})
	if err := open.controlCheckRedirect(makeReq("https://evil.example/steal"), []*http.Request{first}); err == nil {
		t.Fatal("empty allowlist allowed a cross-origin control redirect")
	}
	chain := make([]*http.Request, 4)
	for i := range chain {
		chain[i] = first
	}
	if err := open.controlCheckRedirect(makeReq("https://provider.example/next"), chain); err == nil {
		t.Fatal("control client accepted more than three redirects")
	}
	if err := a.private.controlCheckRedirect(makeReq("https://provider.example/next"), []*http.Request{first}); err != nil {
		t.Fatalf("same-origin redirect rejected: %v", err)
	}
	mediaCheck := a.private.newMediaClient().CheckRedirect
	for _, target := range []string{"file:///etc/passwd", "https://cdn.example:9443/asset"} {
		if err := mediaCheck(makeReq(target), []*http.Request{first}); err == nil {
			t.Errorf("media client accepted redirect to %s", target)
		}
	}
	if err := mediaCheck(makeReq("https://evil.example:8443/steal"), []*http.Request{first}); err != nil {
		t.Fatalf("dynamic public media redirect rejected: %v", err)
	}
	if err := mediaCheck(makeReq("http://cdn.example:8080/asset"), []*http.Request{first}); err != nil {
		t.Fatalf("standard HTTP media redirect rejected: %v", err)
	}
	if err := mediaCheck(makeReq("https://cdn.example/asset"), []*http.Request{first}); err != nil {
		t.Fatalf("explicitly allowlisted HTTPS CDN redirect rejected: %v", err)
	}
}

func TestMediaRedirectPolicyAllowsDynamicPublicHostsAndDropsSensitiveHeaders(t *testing.T) {
	a := testAPI(nil)
	client := a.private.newMediaClient()
	var calls int
	client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": []string{"https://cdn-unlisted.example:443/asset"}}, Body: io.NopCloser(strings.NewReader("redirect")), Request: r}, nil
		}
		for _, header := range []string{"Referer", "Authorization", "Cookie", "Proxy-Authorization", "X-Session", "X-Private"} {
			if value := r.Header.Get(header); value != "" {
				t.Errorf("redirect forwarded forbidden header %s=%q", header, value)
			}
		}
		if r.Header.Get("Range") != "bytes=1-5" || r.Header.Get("If-Range") != "etag" {
			t.Errorf("redirect did not preserve permitted range headers: %v", r.Header)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("ok")), Request: r}, nil
	})
	req, _ := http.NewRequest(http.MethodGet, "https://provider.example/credential-in-path", nil)
	req.Header.Set("Range", "bytes=1-5")
	req.Header.Set("If-Range", "etag")
	req.Header.Set("Referer", "https://provider.example/credential-in-path")
	req.Header.Set("Authorization", "Bearer private")
	req.Header.Set("Cookie", "session=private")
	req.Header.Set("Proxy-Authorization", "Basic private")
	req.Header.Set("X-Session", "private")
	req.Header.Set("X-Private", "private")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("dynamic public redirect failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || calls != 2 {
		t.Fatalf("unexpected redirect result: status=%d calls=%d", resp.StatusCode, calls)
	}
}

func TestMediaRedirectPolicyBoundsHopsAndRejectsUnsafeTargets(t *testing.T) {
	a := testAPI(nil)
	check := a.private.newMediaClient().CheckRedirect
	first, _ := http.NewRequest(http.MethodGet, "https://provider.example/start", nil)
	for _, target := range []string{
		"file:///etc/passwd", "https://user@cdn.example/asset", "https://cdn.example:9443/asset",
		"https://cdn.example/asset#fragment", "https://[fe80::1%25eth0]/asset",
	} {
		req, _ := http.NewRequest(http.MethodGet, target, nil)
		if err := check(req, []*http.Request{first}); err == nil {
			t.Errorf("accepted unsafe redirect URL %q", target)
		}
	}
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		req, _ := http.NewRequest(method, "https://cdn.example/asset", nil)
		if err := check(req, []*http.Request{first}); err == nil {
			t.Errorf("accepted redirected method %s", method)
		}
	}
	for hops := 1; hops <= 8; hops++ {
		req, _ := http.NewRequest(http.MethodGet, "https://cdn.example/asset", nil)
		via := make([]*http.Request, hops)
		for i := range via {
			via[i] = first
		}
		if err := check(req, via); err != nil {
			t.Errorf("rejected redirect hop %d: %v", hops, err)
		}
	}
	tooMany := make([]*http.Request, 9)
	for i := range tooMany {
		tooMany[i] = first
	}
	req, _ := http.NewRequest(http.MethodGet, "https://cdn.example/asset", nil)
	if err := check(req, tooMany); err == nil {
		t.Fatal("accepted ninth redirect")
	}
}

func TestMediaDNSRejectsEmptyMixedAndMappedAddressesAndPinsDial(t *testing.T) {
	for name, ips := range map[string][]netip.Addr{
		"empty":  nil,
		"mixed":  {netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("10.0.0.4")},
		"mapped": {netip.MustParseAddr("::ffff:8.8.8.8")},
	} {
		dialCalled := false
		_, err := dialSelectedPublicIP(context.Background(), "tcp", "media.example", "443", ips, func(context.Context, string, string) (net.Conn, error) {
			dialCalled = true
			return nil, nil
		})
		if err == nil || dialCalled {
			t.Errorf("%s DNS answer was not rejected before dialing: err=%v dialed=%v", name, err, dialCalled)
		}
	}
	var gotAddress string
	_, err := dialSelectedPublicIP(context.Background(), "tcp", "media.example", "443", []netip.Addr{netip.MustParseAddr("8.8.8.8")}, func(_ context.Context, network, address string) (net.Conn, error) {
		if network != "tcp" {
			t.Errorf("unexpected network %q", network)
		}
		gotAddress = address
		return nil, nil
	})
	if err != nil || gotAddress != "8.8.8.8:443" {
		t.Fatalf("did not dial the previously validated IP: addr=%q err=%v", gotAddress, err)
	}
}

func TestResolveMediaHostHandlesIPLiteralsWithoutDNSNormalization(t *testing.T) {
	for _, tc := range []struct {
		host       string
		want       string
		shouldDial bool
	}{
		{host: "8.8.8.8", want: "8.8.8.8", shouldDial: true},
		{host: "10.0.0.1", want: "10.0.0.1"},
		{host: "::ffff:8.8.8.8", want: "::ffff:8.8.8.8"},
	} {
		ips, err := resolveMediaHost(context.Background(), tc.host)
		if err != nil || len(ips) != 1 || ips[0].String() != tc.want {
			t.Fatalf("literal %q was not parsed directly: ips=%v err=%v", tc.host, ips, err)
		}
		dialCalled := false
		_, err = dialSelectedPublicIP(context.Background(), "tcp", tc.host, "443", ips, func(context.Context, string, string) (net.Conn, error) {
			dialCalled = true
			return nil, nil
		})
		if tc.shouldDial && (err != nil || !dialCalled) {
			t.Errorf("public IPv4 literal was rejected: %v", err)
		}
		if !tc.shouldDial && (err == nil || dialCalled) {
			t.Errorf("unsafe IPv4 literal was not rejected before dialing: err=%v called=%v", err, dialCalled)
		}
	}
}

func TestMediaDialResolvesEachConnectionAndRejectsDNSRebinding(t *testing.T) {
	answers := [][]netip.Addr{
		{netip.MustParseAddr("8.8.8.8")},
		{netip.MustParseAddr("10.0.0.8")},
	}
	lookups, dials := 0, 0
	var dialed []string
	lookup := func(context.Context, string) ([]netip.Addr, error) {
		answer := answers[lookups]
		lookups++
		return answer, nil
	}
	dial := func(_ context.Context, network, address string) (net.Conn, error) {
		dials++
		dialed = append(dialed, network+" "+address)
		return nil, nil
	}
	for attempt := range 2 {
		_, err := dialMediaHostWith(context.Background(), "tcp", "cdn.example", "443", lookup, dial)
		if attempt == 0 && err != nil {
			t.Fatalf("first public resolution was rejected: %v", err)
		}
		if attempt == 1 && err == nil {
			t.Fatal("rebound private resolution was accepted")
		}
	}
	if lookups != 2 || dials != 1 || len(dialed) != 1 || dialed[0] != "tcp 8.8.8.8:443" {
		t.Fatalf("unexpected lookup/dial sequence: lookups=%d dials=%d addresses=%v", lookups, dials, dialed)
	}
}

func TestMediaTransportPreservesTLSHostSNIAndCertificateValidationAcrossRedirect(t *testing.T) {
	cert, roots := mediaTestCertificate(t, []string{"provider.example", "cdn.example"})
	var hosts, serverNames []string
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hosts = append(hosts, strings.TrimSuffix(r.Host, ":443"))
		serverNames = append(serverNames, r.TLS.ServerName)
		if r.URL.Path == "/root" {
			http.Redirect(w, r, "https://cdn.example:443/final", http.StatusFound)
			return
		}
		_, _ = io.WriteString(w, "tls-ok")
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	server.StartTLS()
	defer server.Close()

	a := testAPI(nil)
	client := a.private.newMediaClient()
	transport := client.Transport.(*http.Transport)
	transport.TLSClientConfig = &tls.Config{RootCAs: roots}
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	req, _ := http.NewRequest(http.MethodGet, "https://provider.example:443/root", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("verified TLS redirect failed: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK || string(body) != "tls-ok" {
		t.Fatalf("unexpected secure response: status=%d body=%q err=%v", resp.StatusCode, body, err)
	}
	want := []string{"provider.example", "cdn.example"}
	if !reflect.DeepEqual(hosts, want) || !reflect.DeepEqual(serverNames, want) {
		t.Fatalf("TLS host/SNI changed during redirect: hosts=%v SNI=%v", hosts, serverNames)
	}
}

func mediaTestCertificate(t *testing.T, names []string) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: names[0]},
		NotBefore:    time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true, IsCA: true, DNSNames: names,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	privateKey, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKey})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(parsed)
	return cert, roots
}

func TestMediaHTTPTransportFollowsUnlistedRedirectsAndDropsReferer(t *testing.T) {
	var hosts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := strings.TrimSuffix(r.Host, ":80")
		hosts = append(hosts, host)
		if r.Header.Get("Referer") != "" {
			t.Errorf("transport forwarded Referer to %q", host)
		}
		if r.Header.Get("Range") != "bytes=0-3" {
			t.Errorf("transport dropped Range at %q: %q", host, r.Header.Get("Range"))
		}
		switch host {
		case "provider.example":
			http.Redirect(w, r, "http://cdn-a.example:80/one", http.StatusFound)
		case "cdn-a.example":
			http.Redirect(w, r, "http://cdn-b.example:80/two", http.StatusTemporaryRedirect)
		case "cdn-b.example":
			w.WriteHeader(http.StatusPartialContent)
			_, _ = io.WriteString(w, "data")
		default:
			t.Errorf("unexpected redirect host %q", host)
			http.Error(w, "unexpected host", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	a := testAPI(nil)
	client := a.private.newMediaClient()
	transport := client.Transport.(*http.Transport)
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	req, _ := http.NewRequest(http.MethodGet, "http://provider.example:80/root", nil)
	req.Header.Set("Range", "bytes=0-3")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("real HTTP transport failed to follow simulated redirects: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusPartialContent || string(body) != "data" {
		t.Fatalf("unexpected final response: status=%d body=%q err=%v", resp.StatusCode, body, err)
	}
	if len(hosts) != 3 || hosts[0] != "provider.example" || hosts[1] != "cdn-a.example" || hosts[2] != "cdn-b.example" {
		t.Fatalf("unexpected redirect path: %v", hosts)
	}
}

func TestProviderCallBoundsBodyAndHonorsCancellation(t *testing.T) {
	large := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(strings.Repeat("x", (5<<20)+1)))}, nil
	})}
	s := newPrivateState(Config{AllowedHosts: []string{"provider.example"}, HTTPClient: large})
	if _, err := s.providerCall(context.Background(), "https://provider.example", "u", "p", "", nil); !errors.Is(err, errTooLarge) {
		t.Fatalf("oversized provider response was not bounded: %v", err)
	}
	blocking := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	s = newPrivateState(Config{AllowedHosts: []string{"provider.example"}, HTTPClient: blocking})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := s.providerCall(ctx, "https://provider.example", "u", "p", "", nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("provider request ignored cancellation: %v", err)
	}
}

func TestDialRejectsPrivateAddressBeforeConnecting(t *testing.T) {
	s := newPrivateState(Config{AllowedHosts: []string{"127.0.0.1", "::1"}})
	for _, address := range []string{"127.0.0.1:443", "[::1]:443"} {
		if _, err := s.dialPublic(context.Background(), "tcp", address); err == nil || !strings.Contains(err.Error(), "non-public") {
			t.Errorf("private address %s was not rejected before connect: %v", address, err)
		}
	}

	// An empty optional host list expands hostname eligibility but does not
	// relax the public-address or provider-port checks.
	s = newPrivateState(Config{})
	if _, err := s.dialPublic(context.Background(), "tcp", "127.0.0.1:443"); err == nil || !strings.Contains(err.Error(), "non-public") {
		t.Fatalf("empty host list allowed a private provider address: %v", err)
	}
	if _, err := s.dialPublic(context.Background(), "tcp", "provider.example:8443"); err == nil || !strings.Contains(err.Error(), "port is not allowed") {
		t.Fatalf("empty host list relaxed provider port validation: %v", err)
	}
}

func TestDialSelectedIPRejectsEmptyAndMixedDNSAndPinsPublicAnswer(t *testing.T) {
	dialed := ""
	dial := func(_ context.Context, network, address string) (net.Conn, error) {
		dialed = network + " " + address
		return nil, errors.New("fixture stopped before network")
	}
	for _, ips := range [][]netip.Addr{
		nil,
		{netip.MustParseAddr("203.0.113.8")}, // documentation range
		{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("127.0.0.1")},
	} {
		dialed = ""
		if _, err := dialSelectedIP(context.Background(), "tcp", "provider.example", "443", ips, publicIP, dial); err == nil || dialed != "" {
			t.Errorf("invalid DNS answer reached dial (ips=%v, err=%v, dialed=%q)", ips, err, dialed)
		}
	}
	dialed = ""
	_, err := dialSelectedIP(context.Background(), "tcp", "provider.example", "443", []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("1.0.0.1")}, publicIP, dial)
	if err == nil || dialed != "tcp 1.1.1.1:443" {
		t.Fatalf("dial did not use the first validated DNS answer: err=%v dialed=%q", err, dialed)
	}
}

func TestControlDialValidatesResolvedAddressesBeforeConnecting(t *testing.T) {
	lookup := func(_ context.Context, host string) ([]netip.Addr, error) {
		if host != "provider.example" {
			t.Fatalf("lookup received unexpected host %q", host)
		}
		return nil, nil
	}
	dialCalls := 0
	dial := func(context.Context, string, string) (net.Conn, error) {
		dialCalls++
		return nil, errors.New("fixture dial")
	}
	if _, err := dialPublicHostWith(context.Background(), "tcp", "provider.example", "443", lookup, dial); err == nil || !strings.Contains(err.Error(), "no public address") {
		t.Fatalf("empty DNS answer was accepted: %v", err)
	}
	if dialCalls != 0 {
		t.Fatal("empty DNS answer reached dial")
	}
	lookup = func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("::ffff:127.0.0.1")}, nil
	}
	if _, err := dialPublicHostWith(context.Background(), "tcp", "provider.example", "443", lookup, dial); err == nil || !strings.Contains(err.Error(), "non-public") {
		t.Fatalf("mixed public/private DNS answer was accepted: %v", err)
	}
	if dialCalls != 0 {
		t.Fatal("mixed DNS answer reached dial")
	}
	lookup = func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("3fff::1")}, nil
	}
	if _, err := dialPublicHostWith(context.Background(), "tcp", "provider.example", "443", lookup, dial); err == nil || !strings.Contains(err.Error(), "non-public") {
		t.Fatalf("IPv6 documentation DNS answer was accepted: %v", err)
	}
	if dialCalls != 0 {
		t.Fatal("IPv6 documentation DNS answer reached dial")
	}
	lookup = func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("1.0.0.1")}, nil
	}
	if _, err := dialPublicHostWith(context.Background(), "tcp", "provider.example", "443", lookup, dial); err == nil {
		t.Fatal("fixture dial unexpectedly succeeded")
	}
	if dialCalls != 1 {
		t.Fatalf("valid DNS answer did not dial exactly once: %d", dialCalls)
	}
}

func TestControlTransportDoesNotUseProxy(t *testing.T) {
	s := newPrivateState(Config{})
	transport, ok := s.control.Transport.(*http.Transport)
	if !ok || transport.Proxy != nil {
		t.Fatal("provider control transport can use an environment proxy")
	}
}

func TestMediaDialRejectsPrivateAddressWithoutAllowlistingDynamicHost(t *testing.T) {
	s := newPrivateState(Config{})
	for _, address := range []string{"127.0.0.1:443", "[::1]:443"} {
		if _, err := s.dialMediaPublic(context.Background(), "tcp", address); err == nil || classifyMediaError(err) != diagnosticDestinationRejected {
			t.Errorf("media dial allowed private destination %s: %v", address, err)
		}
	}
}

func TestCatalogParsersValidateExtensionsAndSortEpisodes(t *testing.T) {
	itemsRaw := map[string]any{"_list": []map[string]any{
		{"stream_id": float64(2), "name": "Known", "container_extension": ".MP4"},
		{"stream_id": "3", "name": "Bad ext", "container_extension": "exe"},
		{"stream_id": "../1", "name": "Bad id", "container_extension": "mkv"},
	}}
	items := parseItems(itemsRaw, "movie", "stream_id", "account", []byte("identity"))
	if len(items) != 2 || items[0].ContainerExtension != "mp4" || items[1].ContainerExtension != "" || len(items[0].ContentKey) != 64 {
		t.Fatalf("unexpected parsed catalog: %#v", items)
	}
	var provider map[string]any
	if err := json.Unmarshal([]byte(`{"episodes":{"10":[{"id":"20","title":"S10E1","episode_num":1,"container_extension":"mkv"}],"2":[{"id":"12","title":"S2E3","episode_num":3,"container_extension":"mp4"},{"id":"11","title":"S2E1","episode_num":1,"container_extension":"avi"}]}}`), &provider); err != nil {
		t.Fatal(err)
	}
	episodes := parseEpisodes(provider, "account", []byte("identity"))
	if len(episodes) != 3 || episodes[0].Season != 2 || episodes[0].Episode != 1 || episodes[2].Season != 10 {
		t.Fatalf("unexpected episode parse: %#v", episodes)
	}
}

func TestDiscoveryCatalogLanguageInferenceAndLiteralSearchEscaping(t *testing.T) {
	for _, tc := range []struct{ title, category, want string }{
		{"Película [ES]", "", "es"}, {"ES| Canal", "", "es"}, {"Movie", "English Movies", "en"}, {"Film", "Cinéma Français", "fr"}, {"The Office", "Series", "und"},
		{"Vivir en la ciudad", "Drama", "und"}, {"It", "Drama", "und"}, {"French Connection", "Drama", "und"}, {"No sé de dónde", "Drama", "und"},
	} {
		if got := inferCatalogLanguage(tc.title, tc.category); got != tc.want {
			t.Errorf("inferCatalogLanguage(%q,%q)=%q want %q", tc.title, tc.category, got, tc.want)
		}
	}
	if got := escapeLike(`100%_\film`); got != `100\%\_\\film` {
		t.Fatalf("escapeLike did not escape literal LIKE characters: %q", got)
	}
	q := url.Values{"type": {"movie"}, "search": {"100%_"}, "page": {"2"}, "pageSize": {"30"}}
	kind, categoryID, language, page, pageSize, err := parseCatalogPage(q)
	if err != nil || kind != "movie" || categoryID != "" || language != "" || page != 2 || pageSize != 30 {
		t.Fatalf("unexpected parsed catalog filters: %q %q %q %d %d %v", kind, categoryID, language, page, pageSize, err)
	}
	q.Set("pageSize", "101")
	if _, _, _, _, _, err := parseCatalogPage(q); err == nil {
		t.Fatal("accepted page size over the documented cap")
	}
}

func TestCatalogImageCiphertextIsBoundToOwnerAndItemAndRequiresMatchingMIME(t *testing.T) {
	key := []byte(strings.Repeat("e", 32))
	raw := "https://cdn.example/poster.jpg?token=private"
	sealed, err := encryptCatalogImage(key, "owner-a", "source-a", "movie", "25", raw)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte(raw)) || bytes.Contains(sealed, []byte("private")) {
		t.Fatal("catalog image URL was not encrypted")
	}
	opened, err := decryptCatalogImage(key, "owner-a", "source-a", "movie", "25", sealed)
	if err != nil || opened != raw {
		t.Fatalf("encrypted image URL did not round trip: %v", err)
	}
	for _, owner := range []string{"owner-b", "owner-a"} {
		item := "25"
		if owner == "owner-a" {
			item = "26"
		}
		if _, err := decryptCatalogImage(key, owner, "source-a", "movie", item, sealed); err == nil {
			t.Fatal("catalog image ciphertext was transferable between owners or items")
		}
	}
	png := []byte{137, 80, 78, 71, 13, 10, 26, 10}
	if got := verifiedImageType("image/png", png); got != "image/png" {
		t.Fatalf("valid PNG rejected: %q", got)
	}
	if got := verifiedImageType("text/html", png); got != "" {
		t.Fatalf("mismatched declared image MIME accepted: %q", got)
	}
}

func TestCatalogImageCacheIsBoundedDeduplicatedAndPurgedWithSource(t *testing.T) {
	a := testAPI(nil)
	var calls atomic.Int32
	fetch := func() ([]byte, string, error) {
		calls.Add(1)
		time.Sleep(10 * time.Millisecond)
		return []byte("png-data"), "image/png", nil
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body, typ, err := a.private.cachedImage(context.Background(), "source-a", "owner\x00source-a\x001\x00movie\x0025", fetch)
			if err != nil || string(body) != "png-data" || typ != "image/png" {
				t.Errorf("image cache request failed: %q %q %v", body, typ, err)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("concurrent image requests were not deduplicated: %d fetches", calls.Load())
	}
	if a.private.imageCacheBytes != len("png-data") || len(a.private.imageCache) != 1 {
		t.Fatal("image cache accounting is inconsistent")
	}
	a.private.purgeSourceImages("source-a")
	if len(a.private.imageCache) != 0 || a.private.imageCacheBytes != 0 {
		t.Fatal("source image purge retained cached bytes")
	}
	if body, typ, err := a.private.cachedImage(context.Background(), "source-a", "owner\x00source-a\x001\x00movie\x0025", fetch); err != nil || string(body) != "png-data" || typ != "image/png" {
		t.Fatalf("restored source could not repopulate its image cache: %q %q %v", body, typ, err)
	}
	if calls.Load() != 2 {
		t.Fatalf("restored source did not fetch a fresh image after logout purge: %d fetches", calls.Load())
	}
}

func TestLogoutLeavesPersistentCatalogSyncRunning(t *testing.T) {
	a := testAPI(nil)
	ctx, cancel := context.WithCancel(context.Background())
	src := &source{ID: "source-a", UserID: "user-a"}
	a.private.sources[src.ID] = src
	a.private.catalogSync[src.ID] = &catalogSyncJob{cancel: cancel}
	a.private.mu.Lock()
	a.private.evictSourceSessionLocked(src.ID)
	delete(a.private.sources, src.ID)
	a.private.mu.Unlock()
	select {
	case <-ctx.Done():
		t.Fatal("logout canceled the catalog sync for a persistent source")
	default:
	}
	cancel()
}

func TestCatalogSyncAccountLockSerializesSameProviderAccountAndCancelsWaiter(t *testing.T) {
	a := testAPI(nil)
	ctx := context.Background()
	releaseA, ok := a.private.acquireCatalogAccount(ctx, "same-account")
	if !ok {
		t.Fatal("first account lock was not acquired")
	}
	waitCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, ok := a.private.acquireCatalogAccount(waitCtx, "same-account"); ok {
		t.Fatal("canceled waiter bypassed the account lock")
	}
	acquired := make(chan func(), 1)
	go func() {
		release, ok := a.private.acquireCatalogAccount(context.Background(), "same-account")
		if ok {
			acquired <- release
		}
	}()
	select {
	case <-acquired:
		t.Fatal("concurrent synchronization entered while account lock held")
	case <-time.After(20 * time.Millisecond):
	}
	releaseA()
	select {
	case releaseB := <-acquired:
		releaseB()
	case <-time.After(time.Second):
		t.Fatal("queued account synchronization did not resume")
	}
}

func TestCatalogSyncGlobalLimitBoundsPoolUsage(t *testing.T) {
	a := testAPI(nil)
	release1, ok := a.private.acquireCatalogAccount(context.Background(), "account-one")
	if !ok {
		t.Fatal("first catalog slot was not acquired")
	}
	release2, ok := a.private.acquireCatalogAccount(context.Background(), "account-two")
	if !ok {
		release1()
		t.Fatal("second catalog slot was not acquired")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, ok := a.private.acquireCatalogAccount(ctx, "account-three"); ok {
		t.Fatal("catalog work exceeded the global concurrency limit")
	}
	release1()
	release2()
}

func TestForgettingSourceCancelsItsCatalogSync(t *testing.T) {
	a := testAPI(nil)
	ctx, cancel := context.WithCancel(context.Background())
	src := &source{ID: "source-a"}
	a.private.sources[src.ID] = src
	a.private.catalogSync[src.ID] = &catalogSyncJob{cancel: cancel}
	a.private.mu.Lock()
	a.private.forgetSourceLocked(src.ID)
	delete(a.private.sources, src.ID)
	a.private.mu.Unlock()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("forgetting a source did not cancel its catalog sync")
	}
}

func TestProviderEachDecodesBoundedRowsWithoutRetainingProviderURL(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("action") != "get_vod_streams" {
			t.Fatalf("unexpected catalog action")
		}
		return jsonResponse(200, `[{"stream_id":"25","name":"Film","stream_icon":"https://cdn.example/p?token=secret"},{"stream_id":"26","name":"Other"}]`), nil
	})}
	a := testAPI(client)
	src := &source{Base: "https://provider.example", Username: "alice", Password: "secret"}
	seen := []string{}
	err := a.private.providerEach(context.Background(), src, "get_vod_streams", func(row map[string]any) error { seen = append(seen, toString(row["stream_id"])); return nil })
	if err != nil || strings.Join(seen, ",") != "25,26" {
		t.Fatalf("provider rows failed to stream: %v %v", seen, err)
	}
}

func TestCreateSourceAndItemsUseFixtureAndDoNotExposeCredentials(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		query := r.URL.Query()
		if query.Get("username") != "alice" || query.Get("password") != "secret" {
			t.Errorf("fixture request lost provider credentials")
		}
		switch query.Get("action") {
		case "":
			return jsonResponse(200, `{"user_info":{"auth":1,"status":"Active","max_connections":"1"}}`), nil
		case "get_vod_categories":
			return jsonResponse(200, `[{"category_id":"7","category_name":"Films"}]`), nil
		case "get_vod_streams":
			return jsonResponse(200, `[{"stream_id":"25","name":"Film","container_extension":"mp4","category_id":"7"},{"stream_id":"26","name":"Other","container_extension":"mkv","category_id":"7"}]`), nil
		default:
			t.Fatalf("unexpected provider action %q", query.Get("action"))
			return nil, nil
		}
	})}
	a := testAPI(client)
	create := sessionRequest(http.MethodPost, "/api/v1/sources", "member-a", strings.NewReader(`{"playlistUrl":"https://provider.example/get.php?username=alice&password=secret"}`))
	created := httptest.NewRecorder()
	a.createSource(created, create)
	if created.Code != http.StatusCreated || strings.Contains(created.Body.String(), "secret") || strings.Contains(created.Body.String(), "alice") {
		t.Fatalf("source response exposed credentials or failed: %d %s", created.Code, created.Body.String())
	}
	var view SourceView
	if err := json.Unmarshal(created.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	request := sessionRequest(http.MethodGet, "/api/v1/sources/"+view.ID+"/items?type=movie&categoryId=7&page=2&pageSize=1", "member-a", nil)
	request.SetPathValue("id", view.ID)
	items := httptest.NewRecorder()
	a.getItems(items, request)
	var result struct {
		Items    []CatalogItem `json:"items"`
		Total    int           `json:"total"`
		Page     int           `json:"page"`
		PageSize int           `json:"pageSize"`
	}
	if items.Code != 200 || json.Unmarshal(items.Body.Bytes(), &result) != nil || result.Total != 2 || result.Page != 2 || len(result.Items) != 1 || result.Items[0].ID != "26" {
		t.Fatalf("unexpected page result: %d %s", items.Code, items.Body.String())
	}
}

func TestCreateSourceAllowsAnyProviderWhenHostListIsEmpty(t *testing.T) {
	var calls int
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != "unlisted-provider.example" {
			t.Fatalf("unexpected provider host %q", r.URL.Host)
		}
		if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			t.Fatalf("private session headers were forwarded to provider: %#v", r.Header)
		}
		return jsonResponse(http.StatusOK, `{"user_info":{"auth":1,"status":"Active","max_connections":"1"}}`), nil
	})}
	a := testAPI(client)
	a.private.cfg.AllowedHosts = []string{"", "  "}
	r := sessionRequest(http.MethodPost, "/api/v1/sources", "member-a", strings.NewReader(`{"playlistUrl":"https://unlisted-provider.example/get.php?username=alice&password=secret"}`))
	r.Header.Set("Cookie", "session=private")
	r.Header.Set("Authorization", "Bearer private")
	w := httptest.NewRecorder()
	a.createSource(w, r)
	if w.Code != http.StatusCreated || calls != 1 {
		t.Fatalf("empty host list did not allow a public provider source: status=%d calls=%d body=%s", w.Code, calls, w.Body.String())
	}
}

func TestCreateSourceStillRestrictsHostsWhenListIsConfigured(t *testing.T) {
	var calls int
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return jsonResponse(http.StatusOK, `{"user_info":{"auth":1,"status":"Active"}}`), nil
	})}
	a := testAPI(client)
	r := sessionRequest(http.MethodPost, "/api/v1/sources", "member-a", strings.NewReader(`{"playlistUrl":"https://unlisted-provider.example/get.php?username=alice&password=secret"}`))
	w := httptest.NewRecorder()
	a.createSource(w, r)
	if w.Code != http.StatusBadRequest || calls != 0 {
		t.Fatalf("configured host list did not restrict provider source: status=%d calls=%d body=%s", w.Code, calls, w.Body.String())
	}
}

func TestCreateSourceRejectsInactiveAccountsAndPreservesExistingSource(t *testing.T) {
	cases := []struct {
		name string
		info string
	}{
		{name: "unauthorized", info: `{"user_info":{"auth":0,"status":"Active"}}`},
		{name: "expired", info: `{"user_info":{"auth":1,"status":"Expired"}}`},
		{name: "disabled", info: `{"user_info":{"auth":1,"status":"Disabled"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var providerInfo map[string]any
			if err := json.Unmarshal([]byte(tc.info), &providerInfo); err != nil {
				t.Fatal(err)
			}
			if accountActive(providerInfo) {
				t.Fatalf("accountActive accepted inactive account fixture: %s", tc.info)
			}
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusOK, tc.info), nil
			})}
			a := testAPI(client)
			old := &source{ID: "existing", UserID: "member", ExpiresAt: time.Now().Add(time.Hour)}
			a.private.sources[old.ID] = old
			w := httptest.NewRecorder()
			r := sessionRequest(http.MethodPost, "/api/v1/sources", "member", strings.NewReader(`{"playlistUrl":"https://provider.example/get.php?username=private-user&password=private-password"}`))
			a.createSource(w, r)
			if w.Code != http.StatusBadGateway {
				t.Fatalf("inactive provider account status %d, want 502: %s", w.Code, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "private-user") || strings.Contains(w.Body.String(), "private-password") {
				t.Fatalf("rejection response exposed credentials: %s", w.Body.String())
			}
			if got := a.private.sources[old.ID]; got != old || len(a.private.sources) != 1 {
				t.Fatalf("inactive account replaced existing source: %#v", a.private.sources)
			}
		})
	}
}

func TestSeriesCategoriesItemsAndEpisodesUseDifferentProviderActions(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Query().Get("action") {
		case "get_series_categories":
			return jsonResponse(200, `[{"category_id":"9","category_name":"Drama"}]`), nil
		case "get_series":
			return jsonResponse(200, `[{"series_id":"44","name":"Show","category_id":"9"}]`), nil
		case "get_series_info":
			return jsonResponse(200, `{"episodes":{"1":[{"id":"101","title":"Pilot","episode_num":1,"container_extension":"mp4"}]}}`), nil
		default:
			t.Fatalf("unexpected series provider action: %q", r.URL.Query().Get("action"))
			return nil, nil
		}
	})}
	a := testAPI(client)
	src := &source{ID: "s", UserID: "member", AccountKey: "acct", Base: "https://provider.example", Username: "u", Password: "p", ExpiresAt: time.Now().Add(time.Hour)}
	a.private.sources[src.ID] = src
	itemsReq := sessionRequest(http.MethodGet, "/api/v1/sources/s/items?type=series&categoryId=9", "member", nil)
	itemsReq.SetPathValue("id", "s")
	itemsW := httptest.NewRecorder()
	a.getItems(itemsW, itemsReq)
	if itemsW.Code != 200 || !strings.Contains(itemsW.Body.String(), `"id":"44"`) || !strings.Contains(itemsW.Body.String(), `"contentKey"`) {
		t.Fatalf("series list failed: %d %s", itemsW.Code, itemsW.Body.String())
	}
	episodesReq := sessionRequest(http.MethodGet, "/api/v1/sources/s/series/44/episodes", "member", nil)
	episodesReq.SetPathValue("id", "s")
	episodesReq.SetPathValue("seriesId", "44")
	episodesW := httptest.NewRecorder()
	a.getEpisodes(episodesW, episodesReq)
	if episodesW.Code != 200 || !strings.Contains(episodesW.Body.String(), `"title":"Pilot"`) {
		t.Fatalf("episodes failed: %d %s", episodesW.Code, episodesW.Body.String())
	}
}

func TestPlaybackRefreshesExpiredCatalogBeforeUsingMetadata(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Query().Get("action") {
		case "get_vod_categories":
			return jsonResponse(200, `[{"category_id":"7","category_name":"Films"}]`), nil
		case "get_vod_streams":
			return jsonResponse(200, `[{"stream_id":"25","name":"Film refreshed","container_extension":"mp4","category_id":"7"}]`), nil
		default:
			t.Fatalf("unexpected refresh action: %s", r.URL.Query().Get("action"))
			return nil, nil
		}
	})}
	a := testAPI(client)
	src := &source{ID: "source-a", UserID: "member", Fingerprint: "account", Base: "https://provider.example", Username: "u", Password: "p", MaxConnections: 1, ExpiresAt: time.Now().Add(time.Hour), VerifiedAt: time.Now()}
	a.private.sources[src.ID] = src
	item := CatalogItem{ID: "25", Title: "Old title", Kind: "movie", ContainerExtension: "mkv", CategoryID: "7", ContentKey: strings.Repeat("a", 64)}
	a.private.authorized[src.ID] = map[string]CatalogItem{item.ContentKey: item}
	a.private.catalogs[src.ID+":movie:7"] = catalogData{expires: time.Now().Add(-time.Second), items: map[string][]CatalogItem{"movie:7": {item}}}
	w := httptest.NewRecorder()
	a.createPlayback(w, sessionRequest(http.MethodPost, "/api/v1/playbacks", "member", strings.NewReader(`{"sourceId":"source-a","kind":"movie","itemId":"25"}`)))
	var view PlaybackView
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &view) != nil || view.Title != "Film refreshed" || view.Format != "file" {
		t.Fatalf("playback did not use refreshed metadata: %d %s", w.Code, w.Body.String())
	}
}

func TestPlaybackRootUsesOptionalHostRestriction(t *testing.T) {
	newApp := func(hosts []string) *API {
		a := testAPI(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, nil })})
		a.private.cfg.AllowedHosts = hosts
		src := &source{ID: "source-open", UserID: "member", Fingerprint: "account", Base: "https://unlisted-provider.example", Username: "u", Password: "p", MaxConnections: 1, ExpiresAt: time.Now().Add(time.Hour), VerifiedAt: time.Now()}
		a.private.sources[src.ID] = src
		item := CatalogItem{ID: "25", Title: "Film", Kind: "movie", ContainerExtension: "mp4", CategoryID: "7", ContentKey: strings.Repeat("a", 64)}
		a.private.catalogs[src.ID+":movie:7"] = catalogData{expires: time.Now().Add(time.Minute), items: map[string][]CatalogItem{"movie:7": {item}}}
		a.private.authorized[src.ID] = map[string]CatalogItem{item.ContentKey: item}
		return a
	}
	body := `{"sourceId":"source-open","kind":"movie","itemId":"25"}`
	open := httptest.NewRecorder()
	newApp(nil).createPlayback(open, sessionRequest(http.MethodPost, "/api/v1/playbacks", "member", strings.NewReader(body)))
	if open.Code != http.StatusCreated {
		t.Fatalf("empty host list rejected a source's playback root: %d %s", open.Code, open.Body.String())
	}
	restricted := httptest.NewRecorder()
	newApp([]string{"provider.example"}).createPlayback(restricted, sessionRequest(http.MethodPost, "/api/v1/playbacks", "member", strings.NewReader(body)))
	if restricted.Code != http.StatusBadGateway {
		t.Fatalf("explicit host restriction did not reject playback root: %d %s", restricted.Code, restricted.Body.String())
	}
}

func TestPlaybackLeaseConflictAndOwnership(t *testing.T) {
	a := testAPI(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, nil })})
	src := &source{ID: "source-a", UserID: "member-a", Fingerprint: "same-account", Base: "https://provider.example", Username: "u", Password: "p", MaxConnections: 1, ExpiresAt: time.Now().Add(time.Hour), VerifiedAt: time.Now()}
	a.private.sources[src.ID] = src
	item := CatalogItem{ID: "25", Title: "Film", Kind: "movie", ContainerExtension: "mp4", CategoryID: "7", ContentKey: strings.Repeat("a", 64)}
	a.private.catalogs[src.ID+":movie:7"] = catalogData{expires: time.Now().Add(time.Minute), items: map[string][]CatalogItem{"movie:7": {item}}}
	a.private.authorized[src.ID] = map[string]CatalogItem{item.ContentKey: item}
	body := `{"sourceId":"source-a","kind":"movie","itemId":"25"}`
	first := httptest.NewRecorder()
	a.createPlayback(first, sessionRequest(http.MethodPost, "/api/v1/playbacks", "member-a", strings.NewReader(body)))
	second := httptest.NewRecorder()
	a.createPlayback(second, sessionRequest(http.MethodPost, "/api/v1/playbacks", "member-b", strings.NewReader(body)))
	if first.Code != 201 || second.Code != 404 {
		t.Fatalf("unexpected ownership/lease results: %d %d", first.Code, second.Code)
	}
	secondOwn := httptest.NewRecorder()
	a.createPlayback(secondOwn, sessionRequest(http.MethodPost, "/api/v1/playbacks", "member-a", strings.NewReader(body)))
	if secondOwn.Code != 409 {
		t.Fatalf("expected lease conflict, got %d: %s", secondOwn.Code, secondOwn.Body.String())
	}
}

func TestPlaybackLeaseConflictAcrossUsersSharingProviderAccount(t *testing.T) {
	a := testAPI(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, nil })})
	for _, owner := range []string{"member-a", "member-b"} {
		src := &source{ID: owner, UserID: owner, Fingerprint: "shared-provider-account", Base: "https://provider.example", Username: "u", Password: "p", MaxConnections: 1, ExpiresAt: time.Now().Add(time.Hour), VerifiedAt: time.Now()}
		a.private.sources[src.ID] = src
		item := CatalogItem{ID: "25", Title: "Film", Kind: "movie", ContainerExtension: "mp4", CategoryID: "7", ContentKey: owner + strings.Repeat("a", 64)}
		a.private.catalogs[src.ID+":movie:7"] = catalogData{expires: time.Now().Add(time.Minute), items: map[string][]CatalogItem{"movie:7": {item}}}
		a.private.authorized[src.ID] = map[string]CatalogItem{item.ContentKey: item}
	}
	first := httptest.NewRecorder()
	a.createPlayback(first, sessionRequest(http.MethodPost, "/api/v1/playbacks", "member-a", strings.NewReader(`{"sourceId":"member-a","kind":"movie","itemId":"25"}`)))
	second := httptest.NewRecorder()
	a.createPlayback(second, sessionRequest(http.MethodPost, "/api/v1/playbacks", "member-b", strings.NewReader(`{"sourceId":"member-b","kind":"movie","itemId":"25"}`)))
	if first.Code != 201 || second.Code != 409 {
		t.Fatalf("shared provider lease was not enforced: %d %d %s", first.Code, second.Code, second.Body.String())
	}
}

func TestPlaybackHeartbeatCapsIdleExtensionsAtParentSessionExpiry(t *testing.T) {
	makeRequest := func(absolute time.Time) *http.Request {
		r := sessionRequest(http.MethodPost, "/api/v1/playbacks/p/heartbeat", "member", nil)
		r.SetPathValue("id", "p")
		sess := requestSession(r)
		sess.Absolute = absolute
		return r.WithContext(context.WithValue(r.Context(), sessionKey, sess))
	}
	t.Run("active session", func(t *testing.T) {
		a := testAPI(nil)
		now := time.Now()
		absolute := now.Add(2 * time.Minute)
		src := &source{ID: "s", UserID: "member", ExpiresAt: now.Add(time.Minute), AbsoluteExpiresAt: absolute, Activity: now.Add(-time.Minute)}
		p := &playback{ID: "p", UserID: "member", SourceID: "s", ExpiresAt: now.Add(20 * time.Second)}
		a.private.sources[src.ID], a.private.playbacks[p.ID] = src, p
		w := httptest.NewRecorder()
		a.heartbeatPlayback(w, makeRequest(absolute))
		if w.Code != http.StatusNoContent {
			t.Fatalf("heartbeat failed: %d %s", w.Code, w.Body.String())
		}
		if p.ExpiresAt.After(absolute) || src.ExpiresAt.After(absolute) || !src.Activity.After(now) {
			t.Fatalf("heartbeat did not cap idle expiries at session absolute: playback=%s source=%s activity=%s absolute=%s", p.ExpiresAt, src.ExpiresAt, src.Activity, absolute)
		}
	})
	t.Run("expired session", func(t *testing.T) {
		a := testAPI(nil)
		now := time.Now()
		absolute := now.Add(-time.Second)
		src := &source{ID: "s", UserID: "member", ExpiresAt: now.Add(time.Minute), AbsoluteExpiresAt: absolute, Activity: now.Add(-time.Minute)}
		p := &playback{ID: "p", UserID: "member", SourceID: "s", ExpiresAt: now.Add(time.Minute)}
		a.private.sources[src.ID], a.private.playbacks[p.ID] = src, p
		w := httptest.NewRecorder()
		a.heartbeatPlayback(w, makeRequest(absolute))
		if p.ExpiresAt.After(absolute) || src.ExpiresAt.After(absolute) {
			t.Fatalf("expired parent session was renewed: playback=%s source=%s absolute=%s", p.ExpiresAt, src.ExpiresAt, absolute)
		}
	})
}

func TestMediaProxyPreservesRangeAndRewritesHLS(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Range") != "bytes=2-5" {
			t.Errorf("Range header not forwarded: %q", r.Header.Get("Range"))
		}
		return &http.Response{StatusCode: 206, Header: http.Header{"Content-Type": []string{"video/mp4"}, "Content-Range": []string{"bytes 2-5/10"}, "Accept-Ranges": []string{"bytes"}}, Body: io.NopCloser(strings.NewReader("2345"))}, nil
	})}
	a := testAPI(client)
	src := &source{ID: "s", UserID: "member", Fingerprint: "account", ExpiresAt: time.Now().Add(time.Hour)}
	p := &playback{ID: "p", UserID: "member", SourceID: "s", Fingerprint: "account", ExpiresAt: time.Now().Add(time.Minute), Format: "file", Assets: map[string]string{"asset": "https://provider.example/movie/u/p/1.mp4"}, Ctx: context.Background()}
	a.private.sources[src.ID] = src
	a.private.playbacks[p.ID] = p
	a.private.leases[p.Fingerprint] = map[string]struct{}{p.ID: {}}
	r := sessionRequest(http.MethodGet, "/api/v1/media/p/asset", "member", nil)
	r.SetPathValue("playback", "p")
	r.SetPathValue("asset", "asset")
	r.Header.Set("Range", "bytes=2-5")
	w := httptest.NewRecorder()
	a.proxyMedia(w, r)
	if w.Code != 206 || w.Body.String() != "2345" || w.Header().Get("Content-Range") != "bytes 2-5/10" {
		t.Fatalf("range proxy failed: %d %v %q", w.Code, w.Header(), w.Body.String())
	}
	manifest := []byte("#EXTM3U\n#EXT-X-KEY:METHOD=AES-128,URI=\"key.bin\"\n#EXT-X-MAP:URI=\"init.mp4\"\nsegment.ts\n")
	hls := &playback{ID: "hls", UserID: "member", Assets: map[string]string{}, AssetByURL: map[string]string{}, AssetUsed: map[string]time.Time{}, RootAsset: "root"}
	a.private.playbacks[hls.ID] = hls
	base, _ := url.Parse("https://provider.example/live/user/pass/root.m3u8")
	got, err := a.rewriteManifest(hls, base, manifest)
	if err != nil || !strings.Contains(string(got), "/api/v1/media/hls/") || strings.Contains(string(got), "key.bin") || strings.Contains(string(got), "segment.ts") || len(hls.Assets) != 3 {
		t.Fatalf("manifest not rewritten: %v %s", err, got)
	}
	for range 100 {
		if _, err := a.rewriteManifest(hls, base, manifest); err != nil {
			t.Fatal(err)
		}
	}
	if len(hls.Assets) != 3 {
		t.Fatalf("repeated manifest polling grew asset map to %d", len(hls.Assets))
	}
}

func TestMediaProxyHandlesHeadAndUnsatisfiedRange(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodHead {
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"video/mp4"}, "Content-Length": []string{"100"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
		}
		return &http.Response{StatusCode: 416, Header: http.Header{"Content-Range": []string{"bytes */100"}, "Content-Type": []string{"text/html"}, "Content-Length": []string{"37"}}, Body: io.NopCloser(strings.NewReader("range error"))}, nil
	})}
	a := testAPI(client)
	src := &source{ID: "s", UserID: "member", ExpiresAt: time.Now().Add(time.Hour)}
	p := &playback{ID: "p", UserID: "member", SourceID: "s", ExpiresAt: time.Now().Add(time.Minute), Assets: map[string]string{"asset": "https://provider.example/movie/u/p/1.mp4"}, Ctx: context.Background()}
	a.private.sources[src.ID], a.private.playbacks[p.ID] = src, p
	for _, method := range []string{http.MethodHead, http.MethodGet} {
		r := sessionRequest(method, "/api/v1/media/p/asset", "member", nil)
		r.SetPathValue("playback", "p")
		r.SetPathValue("asset", "asset")
		w := httptest.NewRecorder()
		a.proxyMedia(w, r)
		if method == http.MethodHead && (w.Code != 200 || w.Body.Len() != 0 || w.Header().Get("Content-Length") != "100") {
			t.Errorf("HEAD proxy failed: %d %v %q", w.Code, w.Header(), w.Body.String())
		}
		if method == http.MethodGet && (w.Code != 416 || w.Body.Len() != 0 || w.Header().Get("Content-Range") != "bytes */100" || w.Header().Get("Content-Length") != "0") {
			t.Errorf("416 proxy failed: %d %v %q", w.Code, w.Header(), w.Body.String())
		}
	}
}

func TestExtensionlessGenericMIMEManifestIsRewrittenAndSegmentsStream(t *testing.T) {
	const providerURI = "https://provider.example/movie/u/provider-password/1.ts"
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/playlist" {
			body := "\xef\xbb\xbf \n#EXTM3U\n#EXTINF:3,Live\n" + providerURI + "\n"
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/octet-stream"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/octet-stream"}}, Body: io.NopCloser(strings.NewReader("segment-bytes"))}, nil
	})}
	a := testAPI(client)
	src := &source{ID: "s", UserID: "member", ExpiresAt: time.Now().Add(time.Hour)}
	p := &playback{ID: "p", UserID: "member", SourceID: "s", ExpiresAt: time.Now().Add(time.Minute), Format: "hls", RootAsset: "manifest", Assets: map[string]string{"manifest": "https://provider.example/playlist?id=2", "segment": providerURI}, AssetByURL: map[string]string{}, AssetUsed: map[string]time.Time{}, Ctx: context.Background()}
	a.private.sources[src.ID], a.private.playbacks[p.ID] = src, p
	manifestReq := sessionRequest(http.MethodGet, "/api/v1/media/p/manifest", "member", nil)
	manifestReq.SetPathValue("playback", "p")
	manifestReq.SetPathValue("asset", "manifest")
	manifestW := httptest.NewRecorder()
	a.proxyMedia(manifestW, manifestReq)
	if manifestW.Code != 200 || strings.Contains(manifestW.Body.String(), providerURI) || !strings.Contains(manifestW.Body.String(), "/api/v1/media/p/") || !strings.Contains(manifestW.Body.String(), "#EXTM3U") {
		t.Fatalf("extensionless playlist leaked provider URI or failed to rewrite: %d %s", manifestW.Code, manifestW.Body.String())
	}
	segmentReq := sessionRequest(http.MethodGet, "/api/v1/media/p/segment", "member", nil)
	segmentReq.SetPathValue("playback", "p")
	segmentReq.SetPathValue("asset", "segment")
	segmentW := httptest.NewRecorder()
	a.proxyMedia(segmentW, segmentReq)
	if segmentW.Code != 200 || segmentW.Body.String() != "segment-bytes" {
		t.Fatalf("non-manifest segment was altered: %d %q", segmentW.Code, segmentW.Body.String())
	}
}

func TestHLSRedirectsAndResourcesCanUseUnlistedPublicHosts(t *testing.T) {
	a := testAPI(nil)
	var seen []string
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		seen = append(seen, r.URL.Host+" "+r.Method)
		if r.URL.Host != "provider.example" && r.Header.Get("Referer") != "" {
			t.Errorf("redirect leaked Referer to %s", r.URL.Host)
		}
		response := func(status int, headers http.Header, body string) (*http.Response, error) {
			return &http.Response{StatusCode: status, Header: headers, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
		}
		if r.URL.Host == "provider.example" {
			return response(http.StatusFound, http.Header{"Location": []string{"https://origin-unlisted.example/master.m3u8"}}, "redirect")
		}
		if r.URL.Host == "origin-unlisted.example" {
			manifest := "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=100000\nhttps://variant-unlisted.example/v.m3u8\n" +
				"#EXT-X-KEY:METHOD=AES-128,URI=\"https://key-unlisted.example/key.bin\"\n" +
				"#EXT-X-MAP:URI=\"https://map-unlisted.example/init.mp4\"\n" +
				"https://segment-unlisted.example/segment.ts\n"
			return response(http.StatusOK, http.Header{"Content-Type": []string{"application/vnd.apple.mpegurl"}}, manifest)
		}
		if r.URL.Host == "variant-unlisted.example" {
			return response(http.StatusFound, http.Header{"Location": []string{"https://variant-edge.example/v.m3u8"}}, "redirect")
		}
		if r.URL.Host == "variant-edge.example" {
			return response(http.StatusOK, http.Header{"Content-Type": []string{"application/vnd.apple.mpegurl"}}, "#EXTM3U\nchunk.ts\n")
		}
		if strings.HasSuffix(r.URL.Host, "-unlisted.example") {
			return response(http.StatusFound, http.Header{"Location": []string{"https://" + strings.TrimSuffix(r.URL.Host, "-unlisted.example") + "-edge.example" + r.URL.Path}}, "redirect")
		}
		if r.URL.Host == "segment-edge.example" && r.Header.Get("Range") != "bytes=0-2" {
			t.Errorf("Range was not forwarded to redirected segment: %q", r.Header.Get("Range"))
		}
		return response(http.StatusOK, http.Header{"Content-Type": []string{"application/octet-stream"}}, "asset-bytes")
	})
	a.private.media = a.private.newMediaClient()
	a.private.media.Transport = transport
	src := &source{ID: "s", UserID: "member", ExpiresAt: time.Now().Add(time.Hour)}
	p := &playback{ID: "p", UserID: "member", SourceID: "s", ExpiresAt: time.Now().Add(time.Minute), Format: "hls", RootAsset: "root", Assets: map[string]string{"root": "https://provider.example/root.m3u8"}, AssetByURL: map[string]string{}, AssetUsed: map[string]time.Time{}, Ctx: context.Background()}
	a.private.sources[src.ID], a.private.playbacks[p.ID] = src, p
	getAsset := func(asset string, rangeValue string) *httptest.ResponseRecorder {
		r := sessionRequest(http.MethodGet, "/api/v1/media/p/"+asset, "member", nil)
		r.SetPathValue("playback", "p")
		r.SetPathValue("asset", asset)
		if rangeValue != "" {
			r.Header.Set("Range", rangeValue)
		}
		w := httptest.NewRecorder()
		a.proxyMedia(w, r)
		return w
	}
	root := getAsset("root", "")
	if root.Code != http.StatusOK || !strings.Contains(root.Body.String(), "/api/v1/media/p/") || strings.Contains(root.Body.String(), "unlisted.example") {
		t.Fatalf("redirected HLS manifest was not safely rewritten: code=%d body=%s", root.Code, root.Body.String())
	}
	for _, original := range []string{
		"https://variant-unlisted.example/v.m3u8", "https://key-unlisted.example/key.bin",
		"https://map-unlisted.example/init.mp4", "https://segment-unlisted.example/segment.ts",
	} {
		asset := p.AssetByURL[original]
		if asset == "" {
			t.Fatalf("manifest did not register opaque resource for %s", original)
		}
		rangeValue := ""
		if strings.Contains(original, "segment-") {
			rangeValue = "bytes=0-2"
		}
		w := getAsset(asset, rangeValue)
		if strings.Contains(original, "variant-") {
			if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "/api/v1/media/p/") || strings.Contains(w.Body.String(), "chunk.ts") {
				t.Fatalf("redirected variant manifest was not rewritten: code=%d body=%s", w.Code, w.Body.String())
			}
		} else if w.Code != http.StatusOK || w.Body.String() != "asset-bytes" {
			t.Fatalf("redirected HLS resource failed for %s: code=%d body=%q", original, w.Code, w.Body.String())
		}
	}
	if len(seen) < 10 {
		t.Fatalf("expected root, variant, and HLS resource redirects, saw %d requests: %v", len(seen), seen)
	}
}

func TestNinthMediaRedirectClosesReturnedResponseBody(t *testing.T) {
	a := testAPI(nil)
	var bodies []*trackingReadCloser
	client := a.private.newMediaClient()
	client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := &trackingReadCloser{Reader: strings.NewReader("redirect-body")}
		bodies = append(bodies, body)
		redirect := len(bodies)
		return &http.Response{
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": []string{fmt.Sprintf("https://cdn-%d.example:443/next", redirect)}},
			Body:       body,
			Request:    r,
		}, nil
	})
	a.private.media = client
	src := &source{ID: "s", UserID: "owner", ExpiresAt: time.Now().Add(time.Hour)}
	p := &playback{ID: "p", UserID: "owner", SourceID: src.ID, ExpiresAt: time.Now().Add(time.Minute), Format: "file", RootAsset: "asset", Assets: map[string]string{"asset": "https://provider.example/root.mp4"}, Ctx: context.Background()}
	a.private.sources[src.ID], a.private.playbacks[p.ID] = src, p
	r := sessionRequest(http.MethodGet, "/api/v1/media/p/asset", "owner", nil)
	r.SetPathValue("playback", p.ID)
	r.SetPathValue("asset", "asset")
	w := httptest.NewRecorder()
	a.proxyMedia(w, r)
	if w.Code != http.StatusBadGateway || len(bodies) != 9 {
		t.Fatalf("ninth redirect was not rejected as a bounded upstream error: status=%d responses=%d", w.Code, len(bodies))
	}
	diagnosticRequest := sessionRequest(http.MethodGet, "/api/v1/playbacks/p/diagnostic", "owner", nil)
	diagnosticRequest.SetPathValue("id", p.ID)
	diagnostic := httptest.NewRecorder()
	a.playbackDiagnostic(diagnostic, diagnosticRequest)
	if diagnostic.Code != http.StatusOK || diagnostic.Body.String() != `{"cause":"redirect_limit"}`+"\n" {
		t.Fatalf("ninth redirect diagnostic was not precise and sanitized: %d %s", diagnostic.Code, diagnostic.Body.String())
	}
	for i, body := range bodies {
		if !body.closed {
			t.Errorf("redirect response body %d remained open", i+1)
		}
	}
}

func TestDynamicHLSAssetRemainsIsolatedToItsPlaybackUser(t *testing.T) {
	var calls atomic.Int32
	a := testAPI(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"video/mp2t"}}, Body: io.NopCloser(strings.NewReader("segment")), Request: r}, nil
	})})
	src := &source{ID: "s", UserID: "owner", ExpiresAt: time.Now().Add(time.Hour)}
	p := &playback{ID: "p", UserID: "owner", SourceID: src.ID, ExpiresAt: time.Now().Add(time.Minute), Format: "hls", Assets: map[string]string{}, AssetByURL: map[string]string{}, AssetUsed: map[string]time.Time{}, Ctx: context.Background()}
	a.private.sources[src.ID], a.private.playbacks[p.ID] = src, p
	base, _ := url.Parse("https://provider.example/master.m3u8")
	opaque, err := a.opaqueAsset(p, base, "https://cdn-unlisted.example/segment.ts")
	if err != nil {
		t.Fatalf("could not register dynamic HLS asset: %v", err)
	}
	assetID := opaque[strings.LastIndex(opaque, "/")+1:]
	serve := func(user string) *httptest.ResponseRecorder {
		r := sessionRequest(http.MethodGet, opaque, user, nil)
		r.SetPathValue("playback", p.ID)
		r.SetPathValue("asset", assetID)
		w := httptest.NewRecorder()
		a.proxyMedia(w, r)
		return w
	}
	owner := serve("owner")
	if owner.Code != http.StatusOK || owner.Body.String() != "segment" {
		t.Fatalf("playback owner could not fetch dynamic HLS asset: status=%d body=%q", owner.Code, owner.Body.String())
	}
	other := serve("other-user")
	if other.Code != http.StatusNotFound || calls.Load() != 1 {
		t.Fatalf("foreign user reached dynamic HLS upstream: status=%d upstreamCalls=%d", other.Code, calls.Load())
	}
}

func TestHLSCanReferenceDynamicPublicCDNButRejectUnsafeURLs(t *testing.T) {
	a := testAPI(nil)
	p := &playback{ID: "p", Assets: map[string]string{}, AssetByURL: map[string]string{}, AssetUsed: map[string]time.Time{}}
	a.private.playbacks[p.ID] = p
	base, _ := url.Parse("https://provider.example/live/u/p/master.m3u8")
	asset, err := a.opaqueAsset(p, base, "http://cdn.example:80/segment.ts?token=opaque")
	if err != nil || !strings.Contains(asset, "/api/v1/media/p/") {
		t.Fatalf("allowlisted CDN HLS reference rejected: %q %v", asset, err)
	}
	if _, err := a.opaqueAsset(p, base, "https://user@cdn.example/segment.ts"); err == nil {
		t.Fatal("HLS userinfo was accepted")
	}
	if _, err := a.opaqueAsset(p, base, "http://cdn.example:9090/segment.ts"); err == nil {
		t.Fatal("HLS non-approved port was accepted")
	}
	for _, target := range []string{"file:///etc/passwd", "https://cdn.example/segment.ts#fragment", "https://[fe80::1%25eth0]/segment.ts"} {
		if _, err := a.opaqueAsset(p, base, target); err == nil {
			t.Errorf("unsafe HLS reference was accepted: %q", target)
		}
	}
}

func TestMediaPortsAreBoundedAndAppliedToDial(t *testing.T) {
	for _, raw := range []string{
		"http://media.example/asset", "http://media.example:80/asset", "http://media.example:8000/asset",
		"http://media.example:8080/asset", "http://media.example:8081/asset", "http://media.example:8880/asset",
		"http://media.example:8888/asset", "https://media.example/asset", "https://media.example:443/asset", "https://media.example:8443/asset",
	} {
		u, _ := url.Parse(raw)
		if !validMediaURL(u) {
			t.Errorf("valid media URL rejected: %s", raw)
		}
	}
	for _, raw := range []string{"http://media.example:443/asset", "https://media.example:80/asset", "http://media.example:8443/asset", "https://media.example:8080/asset", "https://media.example:9443/asset"} {
		u, _ := url.Parse(raw)
		if validMediaURL(u) {
			t.Errorf("unapproved media URL accepted: %s", raw)
		}
	}
	for _, port := range []string{"80", "443", "8000", "8080", "8081", "8443", "8880", "8888"} {
		var dialed bool
		conn, err := dialMediaHostWith(context.Background(), "tcp", "media.example", port, func(context.Context, string) ([]netip.Addr, error) {
			return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
		}, func(_ context.Context, network, address string) (net.Conn, error) {
			dialed = true
			if address != net.JoinHostPort("8.8.8.8", port) {
				t.Errorf("dial did not pin the checked public IP: %s", address)
			}
			return nil, errors.New("simulated connect failure")
		})
		if err == nil || conn != nil || !dialed {
			t.Errorf("allowed port %s did not reach dial: conn=%v err=%v", port, conn, err)
		}
	}
	for _, port := range []string{"53", "9090", "8443x"} {
		dialed := false
		_, err := dialMediaHostWith(context.Background(), "tcp", "media.example", port, func(context.Context, string) ([]netip.Addr, error) {
			t.Error("policy rejected port after DNS lookup")
			return nil, nil
		}, func(context.Context, string, string) (net.Conn, error) { dialed = true; return nil, nil })
		if err == nil || dialed || classifyMediaError(err) != diagnosticDestinationRejected {
			t.Errorf("unapproved port %s was not rejected before dialing: err=%v dialed=%v", port, err, dialed)
		}
	}
}

func TestPlaybackDiagnosticIsPrivateAndClearedByRootRetry(t *testing.T) {
	responses := []int{http.StatusNotFound, http.StatusOK}
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		status := responses[0]
		responses = responses[1:]
		if status == http.StatusNotFound {
			return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("private-password https://secret.example/video?token=private")), Request: r}, nil
		}
		return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"video/mp4"}}, Body: io.NopCloser(strings.NewReader("bytes")), Request: r}, nil
	})}
	a := testAPI(client)
	src := &source{ID: "source", UserID: "owner", ExpiresAt: time.Now().Add(time.Hour)}
	p := &playback{ID: "play", UserID: "owner", SourceID: src.ID, ExpiresAt: time.Now().Add(time.Minute), RootAsset: "root", Format: "file", Assets: map[string]string{"root": "https://provider.example/video/owner/private-password/1.mp4"}, Ctx: context.Background()}
	a.private.sources[src.ID], a.private.playbacks[p.ID] = src, p
	request := func(user string) *httptest.ResponseRecorder {
		r := sessionRequest(http.MethodGet, "/api/v1/media/play/root", user, nil)
		r.SetPathValue("playback", p.ID)
		r.SetPathValue("asset", "root")
		w := httptest.NewRecorder()
		a.proxyMedia(w, r)
		return w
	}
	getDiagnostic := func(user string) *httptest.ResponseRecorder {
		r := sessionRequest(http.MethodGet, "/api/v1/playbacks/play/diagnostic", user, nil)
		r.SetPathValue("id", p.ID)
		w := httptest.NewRecorder()
		a.playbackDiagnostic(w, r)
		return w
	}
	first := request("owner")
	if first.Code != http.StatusBadGateway || strings.Contains(first.Body.String(), "private-password") || strings.Contains(first.Body.String(), "secret.example") {
		t.Fatalf("upstream failure leaked private response material: status=%d body=%s", first.Code, first.Body.String())
	}
	diag := getDiagnostic("owner")
	if diag.Code != http.StatusOK || diag.Body.String() != `{"cause":"upstream_status","upstreamStatus":404}`+"\n" {
		t.Fatalf("wrong sanitized diagnostic: %d %s", diag.Code, diag.Body.String())
	}
	for _, secret := range []string{"private-password", "secret.example", "token=private", "video/owner"} {
		if strings.Contains(diag.Body.String(), secret) {
			t.Fatalf("diagnostic leaked %q: %s", secret, diag.Body.String())
		}
	}
	if foreign := getDiagnostic("other-user"); foreign.Code != http.StatusNotFound {
		t.Fatalf("foreign user read diagnostic: %d %s", foreign.Code, foreign.Body.String())
	}
	if retried := request("owner"); retried.Code != http.StatusOK {
		t.Fatalf("simulated retry failed: %d %s", retried.Code, retried.Body.String())
	}
	if got := getDiagnostic("owner").Body.String(); got != `{"cause":"none"}`+"\n" {
		t.Fatalf("root retry retained stale diagnostic: %s", got)
	}
	p.ExpiresAt = time.Now().Add(-time.Second)
	if expired := getDiagnostic("owner"); expired.Code != http.StatusNotFound {
		t.Fatalf("expired playback diagnostic was readable: %d %s", expired.Code, expired.Body.String())
	}
}

func TestLateMediaFailureFromPriorRootAttemptCannotReplaceRetryDiagnostic(t *testing.T) {
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	var calls atomic.Int32
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if calls.Add(1) == 1 {
			close(firstStarted)
			<-releaseFirst
			return &http.Response{StatusCode: http.StatusNotFound, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("old failure")), Request: r}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"video/mp4"}}, Body: io.NopCloser(strings.NewReader("bytes")), Request: r}, nil
	})}
	a := testAPI(client)
	src := &source{ID: "source", UserID: "owner", ExpiresAt: time.Now().Add(time.Hour)}
	p := &playback{ID: "play", UserID: "owner", SourceID: src.ID, ExpiresAt: time.Now().Add(time.Minute), RootAsset: "root", Format: "file", Assets: map[string]string{"root": "https://provider.example/video.mp4"}, Ctx: context.Background()}
	a.private.sources[src.ID], a.private.playbacks[p.ID] = src, p
	makeRequest := func() *httptest.ResponseRecorder {
		r := sessionRequest(http.MethodGet, "/api/v1/media/play/root", "owner", nil)
		r.SetPathValue("playback", p.ID)
		r.SetPathValue("asset", "root")
		w := httptest.NewRecorder()
		a.proxyMedia(w, r)
		return w
	}
	firstDone := make(chan struct{})
	go func() { defer close(firstDone); makeRequest() }()
	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("first media attempt did not reach simulated upstream")
	}
	if retry := makeRequest(); retry.Code != http.StatusOK {
		t.Fatalf("retry failed: %d %s", retry.Code, retry.Body.String())
	}
	close(releaseFirst)
	select {
	case <-firstDone:
	case <-time.After(time.Second):
		t.Fatal("prior media attempt did not finish")
	}
	diagnosticRequest := sessionRequest(http.MethodGet, "/api/v1/playbacks/play/diagnostic", "owner", nil)
	diagnosticRequest.SetPathValue("id", p.ID)
	diagnostic := httptest.NewRecorder()
	a.playbackDiagnostic(diagnostic, diagnosticRequest)
	if diagnostic.Code != http.StatusOK || diagnostic.Body.String() != `{"cause":"none"}`+"\n" {
		t.Fatalf("late prior failure overwrote retry state: %d %s", diagnostic.Code, diagnostic.Body.String())
	}
}

func TestClassifyMediaErrorsUsesOnlyCoarseCategories(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"deadline", context.DeadlineExceeded, diagnosticTimeout},
		{"dns", &net.DNSError{Err: "secret resolver detail", Name: "private.example"}, diagnosticDNSConnection},
		{"connection", &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}, diagnosticDNSConnection},
		{"truncated response", io.ErrUnexpectedEOF, diagnosticInvalidResponse},
		{"tls", tls.RecordHeaderError{Msg: "secret tls detail"}, diagnosticTLS},
		{"destination", mediaPolicyError{cause: diagnosticDestinationRejected}, diagnosticDestinationRejected},
		{"redirect limit", mediaPolicyError{cause: diagnosticRedirectLimit}, diagnosticRedirectLimit},
		{"unknown", errors.New("private URL with token=secret"), diagnosticUnknown},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := classifyMediaError(test.err); got != test.want {
				t.Fatalf("got %q want %q", got, test.want)
			}
		})
	}
}

func TestReleasingPlaybackCancelsOnlyItsMediaRequest(t *testing.T) {
	started := make(chan struct{})
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		close(started)
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	a := testAPI(client)
	playCtx, cancel := context.WithCancel(context.Background())
	src := &source{ID: "s", UserID: "member", Fingerprint: "account", ExpiresAt: time.Now().Add(time.Hour)}
	p := &playback{ID: "p", UserID: "member", SourceID: "s", Fingerprint: "account", ExpiresAt: time.Now().Add(time.Minute), Assets: map[string]string{"asset": "https://provider.example/movie/u/p/1.mp4"}, Ctx: playCtx, Cancel: cancel}
	a.private.sources[src.ID] = src
	a.private.playbacks[p.ID] = p
	a.private.leases[p.Fingerprint] = map[string]struct{}{p.ID: {}}
	r := sessionRequest(http.MethodGet, "/api/v1/media/p/asset", "member", nil)
	r.SetPathValue("playback", "p")
	r.SetPathValue("asset", "asset")
	done := make(chan struct{})
	go func() { a.proxyMedia(httptest.NewRecorder(), r); close(done) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("media request did not start")
	}
	a.private.mu.Lock()
	a.private.releasePlaybackLocked("p")
	a.private.mu.Unlock()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("media request did not cancel")
	}
}

func TestLocalPostgresSessionCSRFAndLibraryIsolation(t *testing.T) {
	dsn := os.Getenv("WALACTV_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set WALACTV_TEST_DATABASE_URL to an isolated local test database")
	}
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Hostname() == "" {
		t.Fatal("test database URL is invalid")
	}
	host := strings.ToLower(parsed.Hostname())
	endpointIP := net.ParseIP(host)
	if host != "localhost" && (endpointIP == nil || !endpointIP.IsLoopback()) {
		t.Skip("database endpoint is not loopback; refusing to run mutating test")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal("could not open local test database")
	}
	defer db.Close()
	if err := db.Ping(ctx); err != nil {
		t.Fatal("local test database is not reachable")
	}

	users := make([]user, 2)
	password := "temporary-local-test-password"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	for i := range users {
		id, err := randomToken(12)
		if err != nil {
			t.Fatal(err)
		}
		name := "api-test-" + id
		if _, err := db.Exec(ctx, "INSERT INTO users(id,username,password_hash,role) VALUES($1,$2,$3,'admin')", id, name, string(hash)); err != nil {
			t.Fatal("test schema is not initialized")
		}
		users[i] = user{ID: id, Username: name, Role: "admin"}
	}
	defer db.Exec(ctx, "DELETE FROM users WHERE id=ANY($1)", []string{users[0].ID, users[1].ID})

	a := New(Config{DB: db, Origin: "http://127.0.0.1:8081", SessionSecret: []byte(strings.Repeat("s", 32)), IdentityKey: []byte(strings.Repeat("k", 32)), EncryptionKey: []byte(strings.Repeat("e", 32))})
	type loginResult struct {
		cookie *http.Cookie
		csrf   string
	}
	login := func(u user) loginResult {
		body, _ := json.Marshal(map[string]string{"username": u.Username, "password": password})
		r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(string(body)))
		r.Header.Set("Origin", "http://127.0.0.1:8081")
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("login failed: %d", w.Code)
		}
		var payload struct {
			CSRF string `json:"csrfToken"`
		}
		if json.Unmarshal(w.Body.Bytes(), &payload) != nil || payload.CSRF == "" {
			t.Fatal("login omitted CSRF token")
		}
		var cookie *http.Cookie
		for _, candidate := range w.Result().Cookies() {
			if candidate.Name == "walactv_session" {
				cookie = candidate
			}
		}
		if cookie == nil || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Secure {
			t.Fatal("local session cookie flags are incorrect")
		}
		return loginResult{cookie, payload.CSRF}
	}
	first, second := login(users[0]), login(users[1])
	call := func(method, target string, auth loginResult, body string, csrf bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, target, strings.NewReader(body))
		r.AddCookie(auth.cookie)
		if csrf {
			r.Header.Set("X-CSRF-Token", auth.csrf)
			r.Header.Set("Origin", "http://127.0.0.1:8081")
		}
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, r)
		return w
	}
	if w := call(http.MethodGet, "/api/v1/auth/me", first, "", false); w.Code != 200 {
		t.Fatalf("session lookup failed: %d", w.Code)
	}
	key := strings.Repeat("b", 64)
	a.private.sources["source-a"] = &source{ID: "source-a", UserID: users[0].ID, ExpiresAt: time.Now().Add(time.Hour)}
	a.private.authorized["source-a"] = map[string]CatalogItem{key: {ContentKey: key, Title: "Authorized film", Kind: "movie"}}
	badCSRF := call(http.MethodPost, "/api/v1/favorites", first, `{"contentKey":"`+key+`","title":"Fake","kind":"movie"}`, false)
	if badCSRF.Code != 403 {
		t.Fatalf("mutation without CSRF/origin passed: %d", badCSRF.Code)
	}
	favorite := call(http.MethodPost, "/api/v1/favorites", first, `{"contentKey":"`+key+`","title":"Client title","kind":"movie"}`, true)
	if favorite.Code != 201 {
		t.Fatalf("favorite write failed: %d %s", favorite.Code, favorite.Body.String())
	}
	progress := call(http.MethodPut, "/api/v1/progress", first, `{"contentKey":"`+key+`","title":"Client title","kind":"movie","positionSeconds":12.5,"durationSeconds":90}`, true)
	if progress.Code != 200 {
		t.Fatalf("progress write failed: %d %s", progress.Code, progress.Body.String())
	}
	for _, endpoint := range []string{"/api/v1/favorites", "/api/v1/progress"} {
		own := call(http.MethodGet, endpoint, first, "", false)
		other := call(http.MethodGet, endpoint, second, "", false)
		if own.Code != 200 || !strings.Contains(own.Body.String(), key) {
			t.Fatalf("owner did not read library endpoint %s: %d", endpoint, own.Code)
		}
		if other.Code != 200 || strings.Contains(other.Body.String(), key) {
			t.Fatalf("library leaked across users at %s: %d", endpoint, other.Code)
		}
	}
	me := call(http.MethodGet, "/api/v1/auth/me", first, "", false)
	if me.Code != 200 {
		t.Fatalf("auth/me failed: %d", me.Code)
	}
	logout := call(http.MethodPost, "/api/v1/auth/logout", first, "", true)
	if logout.Code != 204 {
		t.Fatalf("logout failed: %d", logout.Code)
	}
	if w := call(http.MethodGet, "/api/v1/auth/me", first, "", false); w.Code != 401 {
		t.Fatalf("revoked session still active: %d", w.Code)
	}
}
