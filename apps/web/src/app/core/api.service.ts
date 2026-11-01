import { HttpClient, HttpHeaders, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { AceChannel, CatalogKind, CatalogLanguage, CatalogPage, CatalogStatus, Category, Episode, Favorite, MetadataResponse, Playback, PlaybackDiagnostic, PlaybackKind, Progress, Session, Source, TranscodePlayback, VodTracks } from '../models';

const BASE = '/api/v1';

@Injectable({ providedIn: 'root' })
export class ApiService {
  private readonly http = inject(HttpClient);
  private csrf = '';

  setCsrf(token: string): void { this.csrf = token; }
  clearCsrf(): void { this.csrf = ''; }
  private get writeHeaders(): HttpHeaders { return new HttpHeaders({ 'X-CSRF-Token': this.csrf }); }

  login(username: string, password: string): Observable<Session> {
    return this.http.post<Session>(`${BASE}/auth/login`, { username, password });
  }
  me(): Observable<Session> { return this.http.get<Session>(`${BASE}/auth/me`); }
  logout(): Observable<void> { return this.http.post<void>(`${BASE}/auth/logout`, {}, { headers: this.writeHeaders }); }

  sources(): Observable<{ sources: Source[] }> { return this.http.get<{ sources: Source[] }>(`${BASE}/sources`); }
  connect(playlistUrl: string): Observable<Source> {
    return this.http.post<Source>(`${BASE}/sources`, { playlistUrl }, { headers: this.writeHeaders });
  }
  removeSource(id: string): Observable<void> {
    return this.http.delete<void>(`${BASE}/sources/${encodeURIComponent(id)}`, { headers: this.writeHeaders });
  }
  categories(sourceId: string, type: CatalogKind, language = ''): Observable<{ categories: Category[] }> {
    let params = new HttpParams().set('type', type);
    if (language) params = params.set('language', language);
    return this.http.get<{ categories: Category[] }>(`${BASE}/sources/${encodeURIComponent(sourceId)}/categories`, { params });
  }
  catalogStatus(sourceId: string): Observable<CatalogStatus> {
    return this.http.get<CatalogStatus>(`${BASE}/sources/${encodeURIComponent(sourceId)}/catalog`);
  }
  refreshCatalog(sourceId: string): Observable<CatalogStatus> {
    return this.http.post<CatalogStatus>(`${BASE}/sources/${encodeURIComponent(sourceId)}/catalog/refresh`, {}, { headers: this.writeHeaders });
  }
  languages(sourceId: string, type: CatalogKind): Observable<{ languages: CatalogLanguage[] }> {
    return this.http.get<{ languages: CatalogLanguage[] }>(`${BASE}/sources/${encodeURIComponent(sourceId)}/languages`, { params: { type } });
  }
  items(sourceId: string, type: CatalogKind, categoryId: string, language: string, page: number, search: string): Observable<CatalogPage> {
    let params = new HttpParams().set('type', type).set('page', page).set('pageSize', 40);
    if (categoryId) params = params.set('categoryId', categoryId);
    if (language) params = params.set('language', language);
    if (search.trim()) params = params.set('search', search.trim());
    return this.http.get<CatalogPage>(`${BASE}/sources/${encodeURIComponent(sourceId)}/items`, { params });
  }
  item(sourceId: string, kind: CatalogKind, itemId: string): Observable<import('../models').CatalogItem> {
    return this.http.get<import('../models').CatalogItem>(`${BASE}/sources/${encodeURIComponent(sourceId)}/items/${kind}/${encodeURIComponent(itemId)}`);
  }
  episodes(sourceId: string, seriesId: string): Observable<{ episodes: Episode[] }> {
    return this.http.get<{ episodes: Episode[] }>(`${BASE}/sources/${encodeURIComponent(sourceId)}/series/${encodeURIComponent(seriesId)}/episodes`);
  }
  metadata(sourceId: string, kind: 'movie'|'series', itemId: string): Observable<MetadataResponse> {
    return this.http.get<MetadataResponse>(`${BASE}/sources/${encodeURIComponent(sourceId)}/items/${kind}/${encodeURIComponent(itemId)}/metadata`);
  }
  seasonMetadata(sourceId: string, itemId: string, season: number): Observable<MetadataResponse> {
    return this.http.get<MetadataResponse>(`${BASE}/sources/${encodeURIComponent(sourceId)}/items/series/${encodeURIComponent(itemId)}/seasons/${season}/metadata`);
  }
  openPlayback(sourceId: string, kind: PlaybackKind, itemId: string, seriesId?: string): Observable<Playback> {
    return this.http.post<Playback>(`${BASE}/playbacks`, { sourceId, kind, itemId, ...(seriesId ? { seriesId } : {}) }, { headers: this.writeHeaders });
  }
  heartbeatPlayback(id: string): Observable<void> {
    return this.http.post<void>(`${BASE}/playbacks/${encodeURIComponent(id)}/heartbeat`, {}, { headers: this.writeHeaders });
  }
  playbackDiagnostic(id: string): Observable<PlaybackDiagnostic> {
    return this.http.get<PlaybackDiagnostic>(`${BASE}/playbacks/${encodeURIComponent(id)}/diagnostic`);
  }
  playbackTracks(id: string): Observable<VodTracks> {
    return this.http.post<VodTracks>(`${BASE}/playbacks/${encodeURIComponent(id)}/tracks`, {}, { headers: this.writeHeaders });
  }
  transcodePlayback(id: string, offsetSeconds: number, selection?: { trackRevision?: string; audioTrackId?: string; subtitleTrackId?: string }): Observable<TranscodePlayback> {
    return this.http.post<TranscodePlayback>(`${BASE}/playbacks/${encodeURIComponent(id)}/transcode`, { offsetSeconds, ...selection }, { headers: this.writeHeaders });
  }
  cancelTranscode(id: string): Observable<void> {
    return this.http.delete<void>(`${BASE}/playbacks/${encodeURIComponent(id)}/transcode`, { headers: this.writeHeaders });
  }
  closePlayback(id: string): Observable<void> {
    return this.http.delete<void>(`${BASE}/playbacks/${encodeURIComponent(id)}`, { headers: this.writeHeaders });
  }
  favorites(): Observable<{ favorites: Favorite[] }> { return this.http.get<{ favorites: Favorite[] }>(`${BASE}/favorites`); }
  addFavorite(contentKey: string, title: string, kind: PlaybackKind): Observable<Favorite> {
    return this.http.post<Favorite>(`${BASE}/favorites`, { contentKey, title, kind }, { headers: this.writeHeaders });
  }
  removeFavorite(id: string): Observable<void> { return this.http.delete<void>(`${BASE}/favorites/${encodeURIComponent(id)}`, { headers: this.writeHeaders }); }
  progress(): Observable<{ progress: Progress[] }> { return this.http.get<{ progress: Progress[] }>(`${BASE}/progress`); }
  saveProgress(contentKey: string, title: string, kind: PlaybackKind, positionSeconds: number, durationSeconds: number): Observable<Progress> {
    return this.http.put<Progress>(`${BASE}/progress`, { contentKey, title, kind, positionSeconds, durationSeconds }, { headers: this.writeHeaders });
  }
  aceChannels(): Observable<{ channels: AceChannel[] }> { return this.http.get<{ channels: AceChannel[] }>(`${BASE}/acestream/channels`); }
}
