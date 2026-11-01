import { Component, DestroyRef, ElementRef, OnDestroy, OnInit, ViewChild, inject, signal } from '@angular/core';
import { HttpErrorResponse } from '@angular/common/http';
import { ActivatedRoute, Router } from '@angular/router';
import { firstValueFrom, filter } from 'rxjs';
import { NavigationEnd } from '@angular/router';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import type Hls from 'hls.js';
import { ApiService } from '../../core/api.service';
import { PlaybackStateService } from '../../core/playback-state.service';
import type { CatalogItem, CatalogKind, Episode, Playback, PlaybackKind, Source, VodTrack } from '../../models';
import { ContentRoutes } from '../../core/content-routes';
import { AuthService } from '../../core/auth.service';

type NativeAudioTrack = { language: string; label: string; enabled: boolean };
type NativeAudioTrackList = { length: number; [index: number]: NativeAudioTrack; addEventListener(type: string, listener: () => void): void; removeEventListener(type: string, listener: () => void): void };
type WebkitFullscreenVideo = HTMLVideoElement & {
  webkitEnterFullscreen?: () => void;
  webkitExitFullscreen?: () => void;
  webkitDisplayingFullscreen?: boolean;
};

@Component({
  standalone: true,
  template: `
    <main class="player-page" [class.live-player-page]="playbackState.kind() === 'live'" [class.vod-player-page]="playbackState.kind() !== 'live'">
      <header><div class="brand" aria-label="WalacTV"><span class="brand-mark"><img src="/logo.png" alt="" width="36" height="19"></span><span>walac<span>tv</span></span></div></header>
      <section class="player-content">
        <button class="back-link" (click)="exit()"><svg aria-hidden="true"><use href="/icons.svg#arrow-left"/></svg><span>Volver al catálogo</span></button>
        <div class="player-title"><div>@if (playbackState.kind() !== 'live') { <p class="eyebrow"><span></span>{{ playbackState.kind() === 'movie' ? 'PELÍCULA' : 'EPISODIO' }}</p> }<h1>{{ playbackState.title() }}</h1></div>@if (playbackState.kind() === 'live') { <span class="live-badge"><i></i> En directo</span> }</div>
        <div #playerSurface class="playback-surface" [class.live-playback-surface]="playbackState.kind() === 'live'" [class.vod-playback-surface]="playbackState.kind() !== 'live'" [class.controls-hidden]="!controlsVisible()" (fullscreenchange)="syncFullscreen()" (mousemove)="revealControls()" (touchstart)="revealControls()" (keydown)="revealControls()" (focusin)="revealControls()" (focusout)="scheduleControlHide()">
        <div class="video-shell" [class.is-loading]="loading()">
          <video #video [controls]="playbackState.kind() === 'live' || nativeFullscreen()" playsinline preload="metadata" (loadedmetadata)="onMetadata()" (timeupdate)="onTimeUpdate()" (waiting)="loading.set(true)" (playing)="playing()" (pause)="paused()" (volumechange)="syncVolume()" (ended)="onEnded()" (error)="onMediaError()" (webkitbeginfullscreen)="setWebkitFullscreen(true)" (webkitendfullscreen)="setWebkitFullscreen(false)"></video>
          @if (loading() && !failed()) { <div class="video-message loading-message"><span class="loader"></span><b>Preparando la reproducción</b></div> }
          @if (needsTap() && !failed()) { <button class="tap-to-play" (click)="play()" aria-label="Reproducir"><span aria-hidden="true"><svg><use href="/icons.svg#play"/></svg></span><b>Reproducir</b></button> }
          @if (failed()) { <div class="video-message failure-message"><span>!</span><b>{{ failureTitle() }}</b><small>{{ failureDetail() }}</small>@if (canOfferTranscode()) { <button (click)="startTranscode(true)" [disabled]="transcodeBusy()">{{ transcodeBusy() ? 'Preparando versión compatible…' : 'Probar versión compatible (hasta 720p)' }}</button> }<button class="secondary-action" (click)="retry()">Reintentar</button></div> }
        </div>
        @if (transcoding()) {
          <div class="transcode-status">
            <span>Versión compatible · hasta 720p</span>
            @if (transcodeDuration() > 0) {
              <label class="timeline"><span>{{ formatTime(logicalPosition()) }}</span><input aria-label="Buscar en la película" type="range" min="0" [max]="transcodeDuration()" step="1" [value]="Math.min(logicalPosition(), transcodeDuration())" [disabled]="transcodeBusy()" (change)="seekTranscode($event)"><span>{{ formatTime(transcodeDuration()) }}</span></label>
            } @else { <small>Duración desconocida. La barra de búsqueda no está disponible.</small> }
            <small class="pause-policy">Si pausas más de 45 segundos, al reanudar se prepara un flujo nuevo desde la posición pausada.</small>
            <div class="transcode-controls" role="group" aria-label="Controles del reproductor">
              <button type="button" [attr.aria-label]="isPlaying() ? 'Pausar' : 'Reproducir'" [disabled]="failed()" (click)="togglePlayback()"><svg class="player-icon" aria-hidden="true"><use [attr.href]="isPlaying() ? '/icons.svg#pause' : '/icons.svg#play'"/></svg></button>
              <button type="button" [attr.aria-label]="muted() ? 'Activar sonido' : 'Silenciar'" [attr.aria-pressed]="muted()" (click)="toggleMute()"><svg class="player-icon" aria-hidden="true"><use [attr.href]="muted() ? '/icons.svg#volume-off' : '/icons.svg#volume'"/></svg></button>
              <label class="volume-control"><span>Volumen</span><input aria-label="Volumen" type="range" min="0" max="1" step="0.05" [value]="volume()" (input)="setVolume($event)"></label>
              <button type="button" [attr.aria-label]="fullscreen() ? 'Salir de pantalla completa' : 'Pantalla completa'" [attr.aria-pressed]="fullscreen()" (click)="toggleFullscreen()"><svg class="player-icon" aria-hidden="true"><use [attr.href]="fullscreen() ? '/icons.svg#fullscreen-exit' : '/icons.svg#fullscreen'"/></svg></button>
            </div>
          </div>
        }
        @if (playbackState.kind() !== 'live' && !transcoding()) {
          <div class="vod-control-bar" role="group" aria-label="Controles del reproductor">
            <button type="button" [attr.aria-label]="isPlaying() ? 'Pausar' : 'Reproducir'" (click)="togglePlayback()"><svg class="player-icon" aria-hidden="true"><use [attr.href]="isPlaying() ? '/icons.svg#pause' : '/icons.svg#play'"/></svg></button>
            <span>{{ formatTime(logicalPosition()) }}</span>
            @if (nativeDuration() > 0) { <input aria-label="Posición de reproducción" type="range" min="0" [max]="nativeDuration()" step="1" [value]="Math.min(logicalPosition(), nativeDuration())" (change)="seekNative($event)"><span>{{ formatTime(nativeDuration()) }}</span> }
            <button type="button" [attr.aria-label]="muted() ? 'Activar sonido' : 'Silenciar'" [attr.aria-pressed]="muted()" (click)="toggleMute()"><svg class="player-icon" aria-hidden="true"><use [attr.href]="muted() ? '/icons.svg#volume-off' : '/icons.svg#volume'"/></svg></button>
            <label class="volume-control"><span>Volumen</span><input aria-label="Volumen" type="range" min="0" max="1" step="0.05" [value]="volume()" (input)="setVolume($event)"></label>
            <button type="button" [attr.aria-label]="fullscreen() ? 'Salir de pantalla completa' : 'Pantalla completa'" [attr.aria-pressed]="fullscreen()" (click)="toggleFullscreen()"><svg class="player-icon" aria-hidden="true"><use [attr.href]="fullscreen() ? '/icons.svg#fullscreen-exit' : '/icons.svg#fullscreen'"/></svg></button>
          </div>
        }
        @if (playbackState.kind() !== 'live') { <section class="track-picker" aria-label="Pistas de reproducción">
          @if (hasMultipleSelectableAudio()) { <label><svg class="player-icon" aria-hidden="true"><use href="/icons.svg#audio"/></svg><span>Audio</span><select aria-label="Pista de audio" [value]="selectedAudio()" (change)="chooseAudio($event)">@for (track of audioChoices(); track track.key) { <option [value]="track.key" [disabled]="track.disabled">{{ track.label }}</option> }</select></label> }
          @if (subtitleChoices().length > 0) { <label><svg class="player-icon" aria-hidden="true"><use href="/icons.svg#subtitles"/></svg><span>Subtítulos</span><select aria-label="Pista de subtítulos" [value]="selectedSubtitle()" (change)="chooseSubtitle($event)"><option value="off">Desactivados</option>@for (track of subtitleChoices(); track track.key) { <option [value]="track.key" [disabled]="track.disabled">{{ track.label }}</option> }</select></label> }
          @if (isFilePlayback() && !transcoding()) { <button type="button" (click)="inspectTracks()" [disabled]="tracksBusy()"><svg class="player-icon" aria-hidden="true"><use href="/icons.svg#scan"/></svg>{{ tracksBusy() ? 'Buscando pistas…' : 'Buscar pistas del archivo' }}</button> }
          @if (tracksMessage()) { <small role="status">{{ tracksMessage() }}</small> }
        </section> }
        </div>
        @if (playbackState.kind() !== 'live') { <div class="player-details"><div><span>FORMATO ORIGINAL</span><b>{{ playback()?.format === 'hls' ? 'HLS adaptable' : extension() }}</b></div><p>La compatibilidad depende del navegador y de sus códecs de vídeo.</p></div> }
      </section>
    </main>
  `,
  styles: [`
    :host{display:block;min-height:100vh;background:#11120f;color:#f0eee8}.player-page{min-height:100vh;background:radial-gradient(ellipse at 52% 36%,#27291f 0,transparent 48%),#11120f}.player-page>header{height:62px;border-bottom:1px solid #292b24;padding:0 clamp(20px,5vw,70px);display:flex;align-items:center;justify-content:space-between}.brand{display:flex;align-items:center;gap:10px;text-decoration:none;color:#f2f0e8;font-size:16px;font-weight:700;letter-spacing:-.7px}.brand>span:last-child span{color:#c6f268}.brand-mark{width:29px;height:29px;display:grid;place-content:center;border-radius:9px;background:#c6f268;color:#171913;font-weight:800;font-size:19px;letter-spacing:-2px}.brand-mark span{color:#657a37}.session-state{font-size:8px;letter-spacing:1.2px;color:#85877c}.session-state i{display:inline-block;width:5px;height:5px;background:#c6f268;border-radius:50%;margin-right:7px}.player-content{width:min(1060px,calc(100% - 42px));margin:0 auto;padding:35px 0 55px}.back-link{display:flex;gap:9px;align-items:center;background:none;border:0;color:#b3b5aa;font:inherit;font-size:10px;cursor:pointer}.back-link:hover{color:#c6f268}.player-title{display:flex;align-items:end;justify-content:space-between;margin:28px 0 17px}.eyebrow{font-size:8px;letter-spacing:1.4px;color:#b6c784;display:flex;align-items:center;gap:8px;margin:0}.eyebrow span{width:6px;height:6px;border-radius:50%;background:#f27a66;box-shadow:0 0 8px #f27a66}.player-title h1{font:clamp(23px,3.5vw,37px) Georgia,serif;letter-spacing:-.6px;font-weight:400;margin:10px 0 0;max-width:76vw}.stop-button{display:flex;align-items:center;gap:10px;padding:8px 12px;background:#24261f;border:1px solid #3a3d32;border-radius:5px;color:#c4c6bb;font:inherit;font-size:9px;cursor:pointer;white-space:nowrap}.stop-button span{font-size:17px;color:#c6f268}.video-shell{position:relative;aspect-ratio:16/9;max-height:67vh;background:#030403;border:1px solid #36392e;border-radius:8px;overflow:hidden;box-shadow:0 18px 75px #0007}.video-shell video{width:100%;height:100%;display:block;object-fit:contain;background:#030403}.video-shell.is-loading video{opacity:.2}.video-message,.tap-to-play{position:absolute;inset:0;display:flex;flex-direction:column;align-items:center;justify-content:center;text-align:center;background:#05060488;color:#edeee7}.video-message b,.tap-to-play b{font-size:13px;font-weight:550;margin-top:12px}.video-message small,.tap-to-play small{color:#9da095;font-size:10px;margin-top:7px}.loader{width:30px;height:30px;border:2px solid #747c5d;border-top-color:#c6f268;border-radius:50%;animation:spin .8s linear infinite}@keyframes spin{to{transform:rotate(360deg)}}.tap-to-play{border:0;background:#05060462;cursor:pointer;font:inherit}.tap-to-play span{width:56px;height:56px;border-radius:50%;background:#c6f268;color:#1a1d14;display:grid;place-items:center;padding-left:4px;font-size:17px;box-shadow:0 0 35px #c6f26855}.tap-to-play:hover span{transform:scale(1.07)}.failure-message>span{width:36px;height:36px;border-radius:50%;border:1px solid #c77f68;color:#ffb29a;display:grid;place-items:center;font-size:17px}.failure-message button{margin-top:17px;padding:9px 14px;border-radius:5px;background:#c6f268;color:#1a1c15;border:0;font-size:10px;font-weight:650;cursor:pointer}.failure-message button:disabled{opacity:.55;cursor:wait}.failure-message .secondary-action{margin-top:8px;background:#292c24;border:1px solid #45483b;color:#dedfd5}.transcode-status{display:flex;justify-content:space-between;align-items:center;gap:18px;color:#b9c993;font-size:10px;padding:10px 2px}.transcode-status small{color:#92958a}.transcode-status label{display:flex;align-items:center;gap:10px;min-width:min(460px,60%);color:#aeb0a5;font-size:9px}.transcode-status input{flex:1;accent-color:#c6f268;cursor:pointer}.video-corner{position:absolute;top:13px;left:15px;font-size:7px;letter-spacing:1.2px;color:#aeb3a1;background:#10120dbb;padding:6px 8px;border-radius:3px;pointer-events:none}.video-corner i{font-style:normal;color:#c6f268}.player-details{display:grid;grid-template-columns:1fr 1fr 1fr;gap:18px;padding:17px 4px;border-bottom:1px solid #303229}.player-details>div span,.player-details>div b{display:block}.player-details>div span{color:#777a6e;font-size:7px;letter-spacing:1px}.player-details>div b{margin-top:6px;font-size:10px;font-weight:500;color:#d1d2c9}.player-details>p{grid-column:1/-1;color:#777a70;font-size:9px;margin:2px 0 0}@media(max-width:600px){.player-content{width:calc(100% - 28px);padding-top:25px}.player-title{margin:22px 0 13px;align-items:center}.player-title h1{font-size:22px;line-height:1.2}.stop-button{font-size:0;padding:7px}.stop-button span{font-size:18px}.video-shell{aspect-ratio:16/10;max-height:none}.transcode-status{align-items:stretch;flex-direction:column;gap:8px}.transcode-status label{min-width:100%}.player-details{grid-template-columns:1fr 1fr;gap:15px}.player-details>div:last-of-type{display:none}.player-details>p{line-height:1.5}}
  `, `
    .player-icon{width:18px;height:18px;flex:none;stroke:currentColor;fill:none;stroke-width:1.75;stroke-linecap:round;stroke-linejoin:round}.transcode-controls{display:flex;align-items:center;justify-content:flex-end;gap:8px;width:100%;padding:9px 2px}.transcode-controls button{display:grid;place-items:center;width:44px;height:44px;background:#24261f;border:1px solid #3a3d32;border-radius:5px;color:#e4e5dc;padding:8px;font:inherit;font-size:10px;cursor:pointer}.transcode-controls button:hover:not(:disabled){border-color:#c6f268;color:#c6f268}.transcode-controls button:focus-visible,.transcode-controls input:focus-visible{outline:2px solid #c6f268;outline-offset:2px}.transcode-controls button:disabled{opacity:.5;cursor:not-allowed}.volume-control{display:flex;align-items:center;gap:7px;color:#aeb0a5;font-size:9px}.volume-control input{width:88px;accent-color:#c6f268}.timeline{flex:1}.pause-policy{color:#92958a}.playback-surface:fullscreen{display:flex;flex-direction:column;justify-content:center;background:#030403;padding:2vw}.playback-surface:fullscreen .video-shell{max-height:none;flex:1;aspect-ratio:16/9}.playback-surface:fullscreen .video-shell video{max-height:calc(100vh - 180px)}@media(max-width:600px){.transcode-controls{justify-content:space-between;flex-wrap:wrap}.transcode-controls button{padding:8px}.volume-control input{width:65px}.playback-surface:fullscreen{padding:8px}.playback-surface:fullscreen .video-shell{aspect-ratio:16/10}}
    .track-picker{display:flex;align-items:center;justify-content:flex-end;gap:12px;padding:8px 2px;color:#b9c993;font-size:10px}.track-picker label{display:flex;align-items:center;gap:8px}.track-picker select,.track-picker button{min-height:44px;min-width:44px;border:1px solid #3a3d32;border-radius:5px;background:#24261f;color:#eee;padding:6px 9px;font:inherit}.track-picker button{display:flex;align-items:center;justify-content:center;gap:8px;cursor:pointer}.track-picker select:focus-visible,.track-picker button:focus-visible{outline:2px solid #c6f268;outline-offset:2px}.track-picker small{color:#aeb0a5}.vod-control-bar{display:flex;align-items:center;justify-content:space-between;gap:10px;padding:8px 2px;color:#ddd;font-size:10px}.vod-control-bar button{display:grid;place-items:center;width:44px;height:44px;min-width:44px;min-height:44px;border:1px solid #3a3d32;border-radius:5px;background:#24261f;color:#eee;padding:7px;font:inherit;cursor:pointer}.vod-control-bar button:focus-visible,.vod-control-bar input:focus-visible{outline:2px solid #c6f268;outline-offset:2px}.vod-control-bar input[type=range]{flex:1;min-width:60px;accent-color:#c6f268}.controls-hidden .vod-control-bar,.controls-hidden .track-picker,.controls-hidden .transcode-status{opacity:0;pointer-events:none;transition:opacity .2s ease}@media(max-width:700px){.vod-control-bar{gap:5px;flex-wrap:wrap}.vod-control-bar button{padding:6px}}
  `],
})
export class PlayerComponent implements OnInit, OnDestroy {
  @ViewChild('video') private videoRef?: ElementRef<HTMLVideoElement>;
  @ViewChild('playerSurface') private playerSurfaceRef?: ElementRef<HTMLElement>;
  readonly playbackState = inject(PlaybackStateService);
  private readonly api = inject(ApiService);
  private readonly router = inject(Router);
  private readonly route = inject(ActivatedRoute);
  private readonly contentRoutes = inject(ContentRoutes);
  private readonly auth = inject(AuthService);
  private readonly destroyRef = inject(DestroyRef);
  readonly loading = signal(true);
  readonly failed = signal(false);
  readonly transcoding = signal(false);
  readonly transcodeBusy = signal(false);
  readonly transcodeDuration = signal(0);
  readonly transcodeOffset = signal(0);
  readonly shownPosition = signal(0);
  readonly allowExplicitTranscode = signal(false);
  readonly isPlaying = signal(false);
  readonly muted = signal(false);
  readonly volume = signal(1);
  readonly fullscreen = signal(false);
  readonly nativeFullscreen = signal(false);
  readonly controlsVisible = signal(true);
  readonly audioChoices = signal<Array<{key: string; label: string; disabled?: boolean}>>([]);
  readonly subtitleChoices = signal<Array<{key: string; label: string; disabled?: boolean}>>([]);
  readonly selectedAudio = signal('');
  readonly selectedSubtitle = signal('off');
  readonly tracksBusy = signal(false);
  readonly tracksMessage = signal('');
  readonly Math = Math;
  readonly needsTap = signal(false);
  readonly failureTitle = signal('No se pudo reproducir este contenido');
  readonly failureDetail = signal('Comprueba tu conexión e inténtalo de nuevo.');
  private hls?: Hls;
  private heartbeat?: ReturnType<typeof setInterval>;
  private progressTimer?: ReturnType<typeof setInterval>;
  private progressSave: Promise<void> = Promise.resolve();
  private closing = false;
  private saved = false;
  private mediaAttempt = 0;
  private diagnosticRequestedAttempt = -1;
  private automaticFallbackAttempted = false;
  private transcodeRequested = false;
  private transcodeTask?: Promise<void>;
  private sourcePlayback?: Playback;
  private activeManifest?: string;
  private transcodeGeneration = '';
  private transcodeResumeAfterStart = true;
  private subtitlePoll?: ReturnType<typeof setInterval>;
  private cueAttempt = 0;
  private cueTrack?: TextTrack;
  private controlHideTimer?: ReturnType<typeof setTimeout>;
  private trackRevision = '';
  private inspectedTracks: VodTrack[] = [];
  private transcodeSelection: {trackRevision?: string; audioTrackId?: string; subtitleTrackId?: string} = {};
  private nativeAudioList?: NativeAudioTrackList;
  private nativeTextList?: TextTrackList;
  private readonly nativeTrackChange = () => this.refreshNativeTracks();
  private pausedAt?: number;
  private pausedPosition = 0;
  private readonly longPauseMs = 45_000;
  private routeGeneration = 0;
  private routeKey = '';
  private routeObservedUrl = '';
  private selectionPendingKey = '';
  private pendingSelection?: { sourceId: string; kind: CatalogKind; itemId: string; episodeId: string | null };
  private sourceLookupFailed = false;
  private pendingOpen?: Promise<void>;
  private closeOperation?: Promise<void>;
  private readonly closeOperations = new Map<string, Promise<void>>();
  private legacyAdopting = false;
  private activeSeason?: number;

  private displayPlaybackTitle(title: string, kind: PlaybackKind): string {
    if (kind !== 'live') return title;
    return title.replace(/#{2,}/g, ' ').replace(/\s+/g, ' ').trim() || 'Canal sin nombre';
  }

  ngOnInit(): void {
    const initial = this.route.snapshot;
    void this.handleRouteSelection(String(initial.data['screen'] || ''), String(initial.data['contentKind'] || ''), initial.paramMap.get('sourceId'), initial.paramMap.get('itemId'), initial.paramMap.get('episodeId'));
    this.router.events.pipe(filter((event): event is NavigationEnd => event instanceof NavigationEnd), takeUntilDestroyed(this.destroyRef)).subscribe(() => {
      const snapshot = this.route.snapshot;
      void this.handleRouteSelection(String(snapshot.data['screen'] || ''), String(snapshot.data['contentKind'] || ''), snapshot.paramMap.get('sourceId'), snapshot.paramMap.get('itemId'), snapshot.paramMap.get('episodeId'));
    });
  }

  private activatePlayback(playback: Playback): void {
    this.closing = false;
    this.saved = false;
    this.automaticFallbackAttempted = false;
    this.transcodeRequested = false;
    this.transcoding.set(false); this.transcodeBusy.set(false); this.transcodeDuration.set(0); this.transcodeOffset.set(0); this.shownPosition.set(0);
    this.audioChoices.set([]); this.subtitleChoices.set([]); this.selectedAudio.set(''); this.selectedSubtitle.set('off'); this.tracksMessage.set(''); this.tracksBusy.set(false); this.inspectedTracks = []; this.trackRevision = '';
    this.transcodeSelection = {}; this.transcodeGeneration = ''; this.controlsVisible.set(true);
    this.failed.set(false); this.loading.set(true); this.needsTap.set(false); this.allowExplicitTranscode.set(false);
    this.sourcePlayback = playback;
    void this.initializePlayback(playback);
    if (this.playbackState.kind() !== 'live') {
      this.heartbeat = setInterval(() => this.api.heartbeatPlayback(playback.id).pipe(takeUntilDestroyed(this.destroyRef)).subscribe({ error: error => { if (this.playbackState.playback()?.id === playback.id) this.handleHeartbeatError(error); } }), 10_000);
      this.progressTimer = setInterval(() => { this.progressSave = this.saveProgress(); }, 15_000);
    } else {
      this.heartbeat = setInterval(() => this.api.heartbeatPlayback(playback.id).pipe(takeUntilDestroyed(this.destroyRef)).subscribe({ error: error => { if (this.playbackState.playback()?.id === playback.id) this.handleHeartbeatError(error); } }), 10_000);
    }
  }

  private async handleRouteSelection(screen: string, rawKind: string, sourceId: string | null, itemId: string | null, episodeId: string | null, force = false): Promise<void> {
    const observedUrl = this.router.url;
    if (!force && this.routeObservedUrl === observedUrl) return;
    this.routeObservedUrl = observedUrl;
    if (screen === 'legacy-player') { if (!this.legacyAdopting) await this.adoptLegacyPlayer(); return; }
    this.legacyAdopting = false;
    if (screen !== 'playback') {
      if (!this.playbackState.playback()) void this.router.navigateByUrl('/');
      return;
    }
    const kind = rawKind as CatalogKind;
    const legacy = this.route.snapshot.data['legacy'] === true;
    let parsedItem = legacy ? (itemId && /^[0-9]{1,32}$/.test(itemId) ? { id: itemId, slug: '' } : null)
      : this.contentRoutes.parseRef(this.route.snapshot.paramMap.get(kind === 'series' ? 'seriesRef' : 'itemRef'));
    const parsedEpisode = legacy ? null : this.contentRoutes.parseEpisodeRef(this.route.snapshot.paramMap.get('episodeRef'));
    if (!['live', 'movie', 'series'].includes(kind) || !parsedItem || kind === 'series' && !legacy && !parsedEpisode || kind === 'series' && legacy && (!episodeId || !/^[0-9]{1,32}$/.test(episodeId)) || kind !== 'series' && (parsedEpisode || episodeId)) { this.showUnavailable('La ruta de contenido no es válida.'); return; }
    const pendingSuffix = `:${kind}:${parsedItem.id}:${legacy ? episodeId || '' : parsedEpisode?.id || ''}`;
    if (!legacy) {
      const query = this.router.parseUrl(this.router.url).queryParamMap;
      const keys = Object.keys(this.router.parseUrl(this.router.url).queryParams);
      if (!this.contentRoutes.validOrigin(query.get('o')) || query.getAll('o').length !== 1 || keys.some(value => value !== 'o' && value !== 'temporada') || query.getAll('temporada').length > 1 || query.get('temporada') !== null && (kind !== 'series' || !/^[0-9]{1,6}$/.test(query.get('temporada') || ''))) { await this.rejectRouteSelection('El origen o los parámetros del enlace no son válidos.'); return; }
    } else {
      const query = this.router.parseUrl(this.router.url).queryParamMap;
      const keys = Object.keys(this.router.parseUrl(this.router.url).queryParams);
      if (keys.some(value => value !== 'season') || query.getAll('season').length > 1 || query.get('season') !== null && (kind !== 'series' || !/^[0-9]{1,6}$/.test(query.get('season') || ''))) { await this.rejectRouteSelection('Los parámetros del enlace antiguo no son válidos.'); return; }
    }
    if (!force && this.selectionPendingKey.endsWith(pendingSuffix)) {
      const pendingSource = this.pendingSelection?.sourceId;
      const sameOrigin = legacy ? pendingSource === sourceId
        : !!pendingSource && await this.contentRoutes.origin(pendingSource) === this.route.snapshot.queryParamMap.get('o');
      if (sameOrigin && observedUrl === this.router.url) return;
    }
    const generation = ++this.routeGeneration;
    const activeBeforeSourceCheck = this.playbackState.playback();
    if (activeBeforeSourceCheck && !await this.samePlaybackIdentity(observedUrl)) {
      await this.closeActivePlayback();
      if (generation !== this.routeGeneration || observedUrl !== this.router.url) return;
    }
    let sourcesResult: { sources: Source[] };
    try { sourcesResult = await firstValueFrom(this.api.sources()); }
    catch {
      if (generation !== this.routeGeneration || observedUrl !== this.router.url) return;
      this.sourceLookupFailed = true;
      this.failed.set(true); this.loading.set(false); this.failureTitle.set('No se pudo consultar la fuente');
      this.failureDetail.set('Comprueba la conexión e inténtalo de nuevo.');
      return;
    }
    const retryingSourceLookup = this.sourceLookupFailed;
    this.sourceLookupFailed = false;
    if (generation !== this.routeGeneration || observedUrl !== this.router.url) return;
    const sources = sourcesResult?.sources || [];
    if (sources.length !== 1) { await this.rejectRouteSelection(sources.length ? 'La cuenta tiene más de una fuente activa y no se puede resolver este enlace.' : 'La fuente de este enlace ya no está disponible. Vuelve al catálogo para generar un enlace actual.', generation); return; }
    const source = sources[0];
    if (activeBeforeSourceCheck && this.playbackState.sourceId() !== source.id) await this.closeActivePlayback();
    if (legacy ? source.id !== sourceId : await this.contentRoutes.origin(source.id) !== this.route.snapshot.queryParamMap.get('o')) { await this.rejectRouteSelection('El origen del enlace no coincide con la fuente activa. Vuelve al catálogo para generar un enlace actual.', generation); return; }
    sourceId = source.id;
    itemId = parsedItem.id;
    episodeId = legacy ? episodeId : parsedEpisode?.id || null;
    const desiredPlaybackKind: PlaybackKind = episodeId ? 'episode' : kind === 'series' ? 'episode' : kind;
    const desiredItemId = episodeId || itemId;
    const key = `${sourceId}:${kind}:${itemId}:${episodeId || ''}`;
    const active = this.playbackState.playback();
    if (this.selectionPendingKey === key) return;
    if ((!force || retryingSourceLookup) && this.routeKey === key && active && this.sourcePlayback?.id === active.id) {
      if (retryingSourceLookup) { this.failed.set(false); this.loading.set(false); this.failureTitle.set('No se pudo reproducir este contenido'); this.failureDetail.set('Comprueba tu conexión e inténtalo de nuevo.'); }
      void this.canonicalizeActiveRoute(source, kind, itemId, episodeId, generation);
      return;
    }
    if ((!force || retryingSourceLookup) && active && this.playbackState.sourceId() === sourceId && this.playbackState.kind() === desiredPlaybackKind && this.playbackState.itemId() === desiredItemId && this.playbackState.seriesId() === (episodeId ? itemId : undefined)) {
      if (retryingSourceLookup) { this.failed.set(false); this.loading.set(false); this.failureTitle.set('No se pudo reproducir este contenido'); this.failureDetail.set('Comprueba tu conexión e inténtalo de nuevo.'); }
      this.routeKey = key;
      if (this.sourcePlayback?.id !== active.id) this.activatePlayback(active);
      void this.canonicalizeActiveRoute(source, kind, itemId, episodeId, generation);
      return;
    }
    this.routeKey = key;
    this.selectionPendingKey = key;
    this.pendingSelection = { sourceId, kind, itemId, episodeId };
    this.closing = true;
    await this.closeActivePlayback();
    const pending = this.pendingOpen;
    if (pending) await pending.catch(() => undefined);
    if (generation !== this.routeGeneration) {
      if (this.selectionPendingKey === key) { this.selectionPendingKey = ''; this.pendingSelection = undefined; }
      return;
    }
    this.closing = false;
    const task = this.openSelection(source, kind, itemId, episodeId, parsedEpisode, generation);
    this.pendingOpen = task;
    try { await task; } finally {
      if (this.pendingOpen === task) this.pendingOpen = undefined;
      if (this.selectionPendingKey === key) { this.selectionPendingKey = ''; this.pendingSelection = undefined; }
    }
  }

  private async canonicalizeActiveRoute(source: Source, kind: CatalogKind, itemId: string, episodeId: string | null, generation: number): Promise<void> {
    try {
      const itemKind = kind === 'series' ? 'series' : kind;
      const item = await firstValueFrom(this.api.item(source.id, itemKind, itemId));
      if (generation !== this.routeGeneration) return;
      let destination: string;
      if (kind === 'movie') destination = await this.contentRoutes.movie(source, item, true);
      else if (kind === 'live') destination = await this.contentRoutes.item(source, item);
      else {
        const { episodes } = await firstValueFrom(this.api.episodes(source.id, itemId));
        if (generation !== this.routeGeneration) return;
        const episode = episodes.find(value => value.id === episodeId);
        if (!episode) return;
        destination = await this.contentRoutes.episode(source, item, episode, episode.season);
      }
      if (generation === this.routeGeneration && this.router.url !== destination) {
        await this.router.navigateByUrl(destination, { replaceUrl: true, state: window.history.state });
      }
    } catch { /* keep the active lease and current URL when normalization cannot be resolved */ }
  }

  private async openSelection(source: Source, kind: CatalogKind, itemId: string, episodeId: string | null, _episodeRef: { id: string; slug: string; season: number; episode: number } | null, generation: number): Promise<void> {
    const sourceId = source.id;
    let itemTitle = '';
    let seriesItem: CatalogItem | undefined;
    let selectedEpisode: Episode | undefined;
    let playbackKind: PlaybackKind = kind === 'live' ? 'live' : 'movie';
    let playbackItemId = itemId;
    let seriesId: string | undefined;
    try {
      if (episodeId) {
        const series = await firstValueFrom(this.api.item(sourceId, 'series', itemId));
        if (generation !== this.routeGeneration) return;
        seriesItem = series;
        const { episodes } = await firstValueFrom(this.api.episodes(sourceId, itemId));
        if (generation !== this.routeGeneration) return;
        const episode = episodes.find(value => value.id === episodeId);
        if (!episode) { this.showUnavailable('Este episodio no pertenece a la serie solicitada.'); return; }
        selectedEpisode = episode;
        itemTitle = `${series.title} · T${episode.season} E${episode.episode} · ${episode.title}`;
        this.activeSeason = episode.season;
        playbackKind = 'episode'; playbackItemId = episode.id; seriesId = itemId;
      } else {
        if (kind === 'series') { this.showUnavailable('Para reproducir una serie, elige un episodio.'); return; }
        const item = await firstValueFrom(this.api.item(sourceId, kind, itemId));
        if (generation !== this.routeGeneration) return;
        seriesItem = item;
        itemTitle = item.title;
      }
      if (generation !== this.routeGeneration) return;
      const canonical = kind === 'movie' && seriesItem ? await this.contentRoutes.movie(source, seriesItem, true)
        : kind === 'live' && seriesItem ? await this.contentRoutes.item(source, seriesItem)
        : seriesItem && selectedEpisode ? await this.contentRoutes.episode(source, seriesItem, selectedEpisode, selectedEpisode.season)
        : '';
      if (!canonical) { this.showUnavailable('No se pudo construir el enlace canónico.'); return; }
      if (this.router.url !== canonical) {
        const state = window.history.state;
        await this.router.navigateByUrl(canonical, { replaceUrl: true, state });
        if (generation !== this.routeGeneration || this.closing) return;
      }
      const playback = await firstValueFrom(this.api.openPlayback(sourceId, playbackKind, playbackItemId, seriesId));
      if (generation !== this.routeGeneration || this.closing) {
        await this.cleanupPlayback(playback);
        return;
      }
      this.playbackState.set({ ...playback, title: this.displayPlaybackTitle(playback.title || itemTitle, playbackKind) }, sourceId, playbackKind, playbackItemId, seriesId, 0);
      if (playbackKind !== 'live') {
        try {
          const { progress } = await firstValueFrom(this.api.progress());
          if (generation !== this.routeGeneration || this.playbackState.playback()?.id !== playback.id) {
            await this.cleanupPlayback(playback);
            return;
          }
          this.playbackState.resumePosition.set(progress.find(entry => entry.contentKey === playback.contentKey)?.positionSeconds || 0);
        } catch (error) {
          if (error instanceof HttpErrorResponse && error.status === 401) return;
        }
      }
      if (generation !== this.routeGeneration || this.playbackState.playback()?.id !== playback.id) {
        await this.cleanupPlayback(playback);
        return;
      }
      this.activatePlayback(playback);
    } catch (error) {
      if (generation !== this.routeGeneration) return;
      await this.showPlaybackError(error, sourceId, generation);
    }
  }

  private async adoptLegacyPlayer(): Promise<void> {
    this.legacyAdopting = true;
    const generation = this.routeGeneration;
    const routeUrl = this.router.url;
    const playback = this.playbackState.playback();
    const sourceId = this.playbackState.sourceId();
    const kind = this.playbackState.kind();
    const itemId = this.playbackState.itemId();
    const seriesId = this.playbackState.seriesId();
    if (!playback || !sourceId || !itemId || kind === 'episode' && !seriesId) {
      if (playback) {
        await this.cleanupPlayback(playback, this.transcodeRequested);
        this.playbackState.clear();
      }
      this.noticeLegacyFallback();
      return;
    }
    let result: { sources: Source[] };
    try { result = await firstValueFrom(this.api.sources()); }
    catch {
      if (generation !== this.routeGeneration || routeUrl !== this.router.url) return;
      this.legacyAdopting = false; this.failed.set(true); this.loading.set(false);
      this.failureTitle.set('No se pudo consultar la fuente'); this.failureDetail.set('Comprueba la conexión e inténtalo de nuevo.');
      return;
    }
    if (generation !== this.routeGeneration || routeUrl !== this.router.url) return;
    const sources = result?.sources || [];
    const source = sources.length === 1 && sources[0].id === sourceId ? sources[0] : null;
    if (!source) {
      await this.closeActivePlayback(); this.playbackState.clear();
      if (generation !== this.routeGeneration || routeUrl !== this.router.url) return;
      this.noticeLegacyFallback(); return;
    }
    let destination = '/';
    try {
      const catalogKind: CatalogKind = kind === 'episode' ? 'series' : kind;
      const item = await firstValueFrom(this.api.item(source.id, catalogKind, kind === 'episode' ? seriesId! : itemId));
      if (generation !== this.routeGeneration || routeUrl !== this.router.url) return;
      if (kind === 'live') destination = await this.contentRoutes.item(source, item);
      else if (kind === 'movie') destination = await this.contentRoutes.movie(source, item, true);
      else {
        const { episodes } = await firstValueFrom(this.api.episodes(source.id, seriesId!));
        if (generation !== this.routeGeneration || routeUrl !== this.router.url) return;
        const episode = episodes.find(value => value.id === itemId);
        if (!episode) throw new Error('Episode no longer exists');
        destination = await this.contentRoutes.episode(source, item, episode, episode.season);
      }
    } catch {
      if (generation !== this.routeGeneration || routeUrl !== this.router.url) return;
      await this.closeActivePlayback(); this.playbackState.clear();
      if (generation !== this.routeGeneration || routeUrl !== this.router.url) return;
      this.noticeLegacyFallback(); return;
    }
    if (generation !== this.routeGeneration || routeUrl !== this.router.url) return;
    await this.router.navigateByUrl(destination, { replaceUrl: true, state: this.returnContext(sourceId) });
  }

  private noticeLegacyFallback(): void {
    this.closing = true;
    void this.router.navigateByUrl('/', { replaceUrl: true, state: { notice: 'La ruta antigua no identifica contenido. Elige un elemento del catálogo.' } });
  }

  private returnContext(sourceId: string): Record<string, unknown> {
    const context = (window.history.state as { catalogContext?: { userId?: string; sourceId?: string | null } })?.catalogContext;
    return context && context.userId === this.auth.user()?.id && context.sourceId === sourceId ? { catalogContext: context } : {};
  }

  private showUnavailable(detail: string): void {
    this.failed.set(true); this.loading.set(false); this.failureTitle.set('Este contenido no está disponible'); this.failureDetail.set(detail);
  }

  private async rejectRouteSelection(detail: string, expectedGeneration?: number): Promise<void> {
    const generation = expectedGeneration ?? ++this.routeGeneration;
    if (generation !== this.routeGeneration) return;
    const pending = this.pendingOpen;
    if (this.playbackState.playback()) await this.closeActivePlayback();
    if (pending) await pending.catch(() => undefined);
    if (generation !== this.routeGeneration) return;
    this.selectionPendingKey = ''; this.pendingSelection = undefined;
    this.showUnavailable(detail);
  }

  private async showPlaybackError(error: unknown, sourceId: string, generation: number): Promise<void> {
    this.loading.set(false); this.failed.set(true);
    if (error instanceof HttpErrorResponse && error.status === 404) {
      this.failureTitle.set('Este contenido no está disponible');
      this.failureDetail.set('La fuente, el contenido o la sesión solicitada ya no están disponibles para esta cuenta.');
    } else if (error instanceof HttpErrorResponse && error.status === 503 && error.error?.error?.code === 'CATALOG_NOT_READY') {
      this.failureTitle.set('El catálogo se está preparando'); this.failureDetail.set('Espera a que termine la preparación y vuelve a intentarlo.');
    } else if (error instanceof HttpErrorResponse && error.status === 409 && error.error?.error?.code === 'PLAYBACK_LIMIT') {
      this.failureTitle.set('La cuenta IPTV está ocupada');
      this.failureDetail.set('La cuenta ya tiene una reproducción activa. Cierra la otra reproducción y vuelve a intentarlo.');
    } else if (error instanceof HttpErrorResponse && error.status === 401) {
      this.failureTitle.set('La sesión ha caducado'); this.failureDetail.set('Inicia sesión de nuevo para continuar.');
    } else {
      this.failureTitle.set('No se pudo iniciar la reproducción'); this.failureDetail.set('El servicio no pudo resolver el contenido. Comprueba la conexión e inténtalo de nuevo.');
    }
  }

  playback() { return this.playbackState.playback(); }
  isFilePlayback(): boolean { return this.playback()?.format === 'file'; }
  hasMultipleSelectableAudio(): boolean { return this.audioChoices().filter(track => !track.disabled).length > 1; }
  nativeDuration(): number { const duration = this.videoRef?.nativeElement.duration || 0; return Number.isFinite(duration) ? duration : 0; }
  seekNative(event: Event): void {
    const target = Number((event.target as HTMLInputElement).value);
    const video = this.videoRef?.nativeElement;
    if (!video || !Number.isFinite(target) || target < 0 || target > video.duration) return;
    try { video.currentTime = target; this.shownPosition.set(target); } catch { /* seek range not ready */ }
  }
  revealControls(): void {
    this.controlsVisible.set(true);
    if (this.controlHideTimer) clearTimeout(this.controlHideTimer);
    if (this.isPlaying() && !this.loading() && !this.failed()) this.controlHideTimer = setTimeout(() => this.scheduleControlHide(), 3000);
  }
  scheduleControlHide(): void {
    const surface = this.playerSurfaceRef?.nativeElement;
    if (!this.isPlaying() || this.loading() || this.failed() || surface?.contains(document.activeElement)) { this.controlsVisible.set(true); return; }
    this.controlsVisible.set(false);
  }
  extension(): string { return this.playbackState.itemId().includes('.') ? this.playbackState.itemId().split('.').at(-1)!.toUpperCase() : 'Archivo original'; }
  onMetadata(): void {
    this.loading.set(false);
    const video = this.videoRef?.nativeElement;
    const resumeAt = this.playbackState.resumePosition();
    if (video && resumeAt > 0 && resumeAt < video.duration) {
      try { video.currentTime = resumeAt; this.playbackState.resumePosition.set(0); } catch { /* some HLS streams expose seekable ranges later */ }
    }
  }
  playing(): void {
    if (this.recoverLongPause()) return;
    this.loading.set(false); this.needsTap.set(false); this.isPlaying.set(true);
    this.startSubtitlePolling();
    this.revealControls();
  }
  paused(): void {
    this.isPlaying.set(false);
    this.stopSubtitlePolling();
    this.controlsVisible.set(true);
    if (this.controlHideTimer) clearTimeout(this.controlHideTimer);
    if (this.transcoding() && !this.transcodeBusy() && !this.closing) {
      this.pausedPosition = this.logicalPosition();
      this.pausedAt = Date.now();
    }
  }

  private recoverLongPause(): boolean {
    if (!this.transcoding() || this.pausedAt === undefined || Date.now() - this.pausedAt < this.longPauseMs) {
      this.pausedAt = undefined;
      return false;
    }
    const offset = this.pausedPosition;
    this.pausedAt = undefined;
    this.videoRef?.nativeElement.pause();
    this.startTranscode(true, offset);
    return true;
  }

  async togglePlayback(): Promise<void> {
    const video = this.videoRef?.nativeElement;
    if (!video) return;
    if (video.paused) await this.play();
    else video.pause();
  }

  toggleMute(): void {
    const video = this.videoRef?.nativeElement;
    if (!video) return;
    video.muted = !video.muted;
    this.muted.set(video.muted);
  }

  setVolume(event: Event): void {
    const video = this.videoRef?.nativeElement;
    const value = Number((event.target as HTMLInputElement).value);
    if (!video || !Number.isFinite(value)) return;
    video.volume = Math.max(0, Math.min(1, value));
    if (video.volume > 0) video.muted = false;
    this.volume.set(video.volume);
    this.muted.set(video.muted);
  }

  syncVolume(): void {
    const video = this.videoRef?.nativeElement;
    if (video) { this.volume.set(video.volume); this.muted.set(video.muted); }
  }

  async toggleFullscreen(): Promise<void> {
    const surface = this.playerSurfaceRef?.nativeElement;
    const video = this.videoRef?.nativeElement as WebkitFullscreenVideo | undefined;
    if (!surface || !video) return;
    try {
      if (document.fullscreenElement) await document.exitFullscreen();
      else if (video.webkitDisplayingFullscreen) video.webkitExitFullscreen?.();
      else if (surface.requestFullscreen) await surface.requestFullscreen();
      else if (video.requestFullscreen) await video.requestFullscreen();
      else video.webkitEnterFullscreen?.();
      this.syncFullscreen();
    } catch {
      if (!video.webkitDisplayingFullscreen && video.webkitEnterFullscreen) {
        try { video.webkitEnterFullscreen(); } catch { /* both fullscreen APIs can be unavailable on this device */ }
      }
    }
  }

  syncFullscreen(): void {
    const surface = this.playerSurfaceRef?.nativeElement;
    const video = this.videoRef?.nativeElement as WebkitFullscreenVideo | undefined;
    const nativeVideoFullscreen = !!video && (document.fullscreenElement === video || video.webkitDisplayingFullscreen === true);
    this.nativeFullscreen.set(nativeVideoFullscreen);
    this.fullscreen.set(!!surface && (document.fullscreenElement === surface || nativeVideoFullscreen));
  }

  setWebkitFullscreen(active: boolean): void {
    this.nativeFullscreen.set(active);
    this.fullscreen.set(active);
  }
  onMediaError(): void {
    const video = this.videoRef?.nativeElement;
    if (!video?.currentSrc) return;
    const errorCode = video.error?.code;
    let detail = 'El navegador no pudo reproducir el flujo. La causa concreta todavía no está confirmada.';
    if (errorCode === MediaError.MEDIA_ERR_NETWORK) detail = 'El navegador perdió la conexión con el flujo.';
    if (errorCode === MediaError.MEDIA_ERR_DECODE) detail = 'El navegador no pudo decodificar el flujo recibido. No se ha identificado un códec concreto.';
    if (errorCode === MediaError.MEDIA_ERR_SRC_NOT_SUPPORTED) detail = 'El navegador no admite el formato recibido o el recurso no es válido.';
    if (errorCode === MediaError.MEDIA_ERR_ABORTED) detail = 'El navegador interrumpió la carga del vídeo.';
    this.failAttempt(this.mediaAttempt, 'No se pudo reproducir este archivo', detail, errorCode === MediaError.MEDIA_ERR_DECODE || errorCode === MediaError.MEDIA_ERR_SRC_NOT_SUPPORTED, errorCode === MediaError.MEDIA_ERR_NETWORK || errorCode === MediaError.MEDIA_ERR_ABORTED);
  }
  onTimeUpdate(): void {
    this.loading.set(false);
    const video = this.videoRef?.nativeElement;
    if (!this.transcoding() && video && this.playbackState.resumePosition() > 0 && video.currentTime >= this.playbackState.resumePosition() - 1) this.playbackState.resumePosition.set(0);
    this.shownPosition.set(this.logicalPosition());
  }
  onEnded(): void { void this.exit(); }

  async play(): Promise<void> {
    const video = this.videoRef?.nativeElement; if (!video) return;
    const playbackId = this.playback()?.id;
    if (!playbackId) return;
    if (this.recoverLongPause()) return;
    try { await video.play(); if (this.playback()?.id === playbackId) this.needsTap.set(false); }
    catch { if (this.playback()?.id === playbackId) this.needsTap.set(true); }
  }

  retry(): void {
    if (this.transcoding()) {
      this.startTranscode(true);
      return;
    }
    this.failed.set(false); this.loading.set(true); this.needsTap.set(false);
    void this.handleRouteSelection(String(this.route.snapshot.data['screen'] || ''), String(this.route.snapshot.data['contentKind'] || ''), this.route.snapshot.paramMap.get('sourceId'), this.route.snapshot.paramMap.get('itemId'), this.route.snapshot.paramMap.get('episodeId'), true);
  }

  canOfferTranscode(): boolean {
    return this.allowExplicitTranscode() && !this.transcodeBusy() && !this.transcoding() && this.playbackState.kind() !== 'live' && this.sourcePlayback?.format === 'file';
  }

  logicalPosition(): number {
    const video = this.videoRef?.nativeElement;
    const local = video && Number.isFinite(video.currentTime) ? Math.max(0, video.currentTime) : 0;
    if (this.transcoding()) return this.transcodeOffset() + local;
    return Math.max(local, this.playbackState.resumePosition());
  }

  formatTime(seconds: number): string {
    if (!Number.isFinite(seconds) || seconds < 0) return '—';
    const total = Math.floor(seconds);
    return `${Math.floor(total / 3600) ? `${Math.floor(total / 3600)}:` : ''}${String(Math.floor(total % 3600 / 60)).padStart(2, '0')}:${String(total % 60).padStart(2, '0')}`;
  }

  startTranscode(explicit: boolean, offsetSeconds = this.logicalPosition(), selection?: {trackRevision?: string; audioTrackId?: string; subtitleTrackId?: string}): void {
    if (this.transcodeBusy() || this.closing || (this.automaticFallbackAttempted && !explicit)) return;
    if (!explicit) this.automaticFallbackAttempted = true;
    const task = this.performTranscode(offsetSeconds, selection);
    this.transcodeTask = task;
    void task.finally(() => { if (this.transcodeTask === task) this.transcodeTask = undefined; });
  }

  seekTranscode(event: Event): void {
    const target = Number((event.target as HTMLInputElement).value);
    if (!this.transcoding() || !Number.isFinite(target) || target < 0 || target >= this.transcodeDuration() || this.transcodeBusy()) return;
    const task = this.performTranscode(target);
    this.transcodeTask = task;
    void task.finally(() => { if (this.transcodeTask === task) this.transcodeTask = undefined; });
  }

  async exit(): Promise<void> {
    const snapshot = this.route.snapshot;
    const generation = this.routeGeneration;
    const routeUrl = this.router.url;
    const data = snapshot.data;
    const params = snapshot.paramMap;
    const routeKind = String(data['contentKind'] || '');
    const legacy = data['legacy'] === true;
    const kind = routeKind as CatalogKind;
    const ref = legacy ? null : this.contentRoutes.parseRef(params.get(kind === 'series' ? 'seriesRef' : 'itemRef'));
    const episodeRef = legacy ? null : this.contentRoutes.parseEpisodeRef(params.get('episodeRef'));
    const itemId = legacy ? params.get('itemId') : ref?.id;
    const seriesId = kind === 'series' ? itemId : undefined;
    const episodeId = legacy ? params.get('episodeId') : episodeRef?.id;
    const rawSourceId = legacy ? params.get('sourceId') : this.playbackState.sourceId();
    const sourcesResult = await firstValueFrom(this.api.sources()).catch(() => null);
    if (generation !== this.routeGeneration || routeUrl !== this.router.url) return;
    const sources = sourcesResult?.sources || [];
    const query = this.router.parseUrl(routeUrl).queryParamMap;
    const queryKeys = Object.keys(this.router.parseUrl(routeUrl).queryParams);
    const queryValid = legacy
      ? queryKeys.every(key => key === 'season') && query.getAll('season').length <= 1 && (query.get('season') === null || kind === 'series' && /^[0-9]{1,6}$/.test(query.get('season') || ''))
      : this.contentRoutes.validOrigin(query.get('o')) && query.getAll('o').length === 1 && queryKeys.every(key => key === 'o' || key === 'temporada') && query.getAll('temporada').length <= 1 && (query.get('temporada') === null || kind === 'series' && /^[0-9]{1,6}$/.test(query.get('temporada') || ''));
    const source = queryValid && sources.length === 1 && (!legacy || sources[0].id === rawSourceId)
      && (legacy || await this.contentRoutes.origin(sources[0].id) === query.get('o')) ? sources[0] : null;
    let destination = '/';
    let returnState: Record<string, unknown> = {};
    if (source) {
      returnState = this.returnContext(source.id);
      const origin = await this.contentRoutes.origin(source.id);
      try {
        if (kind === 'movie' && itemId) {
          const item = await firstValueFrom(this.api.item(source.id, 'movie', itemId));
          destination = await this.contentRoutes.movie(source, item);
        } else if (kind === 'series' && seriesId) {
          let season = this.activeSeason;
          const rawSeason = snapshot.queryParamMap.get(legacy ? 'season' : 'temporada');
          try {
            const series = await firstValueFrom(this.api.item(source.id, 'series', seriesId));
            if (episodeId) {
              try {
                const { episodes } = await firstValueFrom(this.api.episodes(source.id, seriesId));
                season = episodes.find(value => value.id === episodeId)?.season;
              } catch { /* use a season already captured from this requested URL */ }
            }
            if (season === undefined && rawSeason !== null && /^[0-9]{1,6}$/.test(rawSeason)) season = Number(rawSeason);
            destination = await this.contentRoutes.series(source, series, season);
          } catch {
            const seriesRef = legacy ? null : this.contentRoutes.parseRef(params.get('seriesRef'));
            if (seriesRef) destination = this.contentRoutes.seriesRef(seriesRef, origin, rawSeason !== null && /^[0-9]{1,6}$/.test(rawSeason) ? Number(rawSeason) : undefined);
          }
        }
      } catch {
        if (kind === 'movie' && !legacy && ref) destination = this.contentRoutes.movieRef(ref, origin);
      }
    }
    if (generation !== this.routeGeneration || routeUrl !== this.router.url) return;
    await this.router.navigateByUrl(destination, { replaceUrl: true, state: returnState });
  }

  async closeForNavigation(): Promise<boolean> {
    this.routeGeneration++;
    await this.closeActivePlayback();
    const pending = this.pendingOpen;
    if (pending) await pending.catch(() => undefined);
    return true;
  }

  async prepareForRoute(nextUrl: string): Promise<boolean> {
    if (await this.samePlaybackIdentity(nextUrl) || await this.samePendingIdentity(nextUrl)) return true;
    return this.closeForNavigation();
  }

  private async samePendingIdentity(url: string): Promise<boolean> {
    const pending = this.pendingSelection;
    if (!pending) return false;
    const parsed = this.router.parseUrl(url);
    const parts = parsed.root.children['primary']?.segments.map(segment => segment.path) || [];
    let sourceMatches = false, kind: CatalogKind | null = null, itemId = '', episodeId: string | null = null;
    try {
      if (parts[0] === 'directo' && parts.length === 2) {
        sourceMatches = await this.contentRoutes.origin(pending.sourceId) === parsed.queryParamMap.get('o');
        kind = 'live'; itemId = this.contentRoutes.parseRef(parts[1])?.id || '';
      } else if (parts[0] === 'peliculas' && parts.length === 3 && parts[2] === 'ver') {
        sourceMatches = await this.contentRoutes.origin(pending.sourceId) === parsed.queryParamMap.get('o');
        kind = 'movie'; itemId = this.contentRoutes.parseRef(parts[1])?.id || '';
      } else if (parts[0] === 'series' && parts.length === 4 && parts[2] === 'episodios') {
        sourceMatches = await this.contentRoutes.origin(pending.sourceId) === parsed.queryParamMap.get('o');
        kind = 'series'; itemId = this.contentRoutes.parseRef(parts[1])?.id || ''; episodeId = this.contentRoutes.parseEpisodeRef(parts[3])?.id || null;
      } else if (parts[0] === 'sources') {
        sourceMatches = parts[1] === pending.sourceId;
        if (parts[2] === 'live' && parts.length === 4) { kind = 'live'; itemId = parts[3]; }
        else if (parts[2] === 'movies' && parts.length === 5 && parts[4] === 'watch') { kind = 'movie'; itemId = parts[3]; }
        else if (parts[2] === 'series' && parts.length === 6 && parts[4] === 'episodes') { kind = 'series'; itemId = parts[3]; episodeId = parts[5]; }
      }
    } catch { return false; }
    return sourceMatches && kind === pending.kind && itemId === pending.itemId && episodeId === pending.episodeId;
  }

  private async samePlaybackIdentity(url: string): Promise<boolean> {
    const sourceId = this.playbackState.sourceId();
    const currentKind = this.playbackState.kind();
    const currentItemId = this.playbackState.itemId();
    const currentSeriesId = this.playbackState.seriesId();
    if (!sourceId || !currentItemId || !this.playbackState.playback()) return false;
    const parsed = this.router.parseUrl(url);
    const parts = parsed.root.children['primary']?.segments.map(segment => segment.path) || [];
    let sourceMatches = false, kind: PlaybackKind | null = null, itemId = '', seriesId: string | undefined;
    try {
      if (parts[0] === 'directo' && parts.length === 2) {
        const ref = this.contentRoutes.parseRef(parts[1]);
        sourceMatches = await this.contentRoutes.origin(sourceId) === parsed.queryParamMap.get('o');
        kind = 'live'; itemId = ref?.id || '';
      } else if (parts[0] === 'peliculas' && parts.length === 3 && parts[2] === 'ver') {
        const ref = this.contentRoutes.parseRef(parts[1]);
        sourceMatches = await this.contentRoutes.origin(sourceId) === parsed.queryParamMap.get('o');
        kind = 'movie'; itemId = ref?.id || '';
      } else if (parts[0] === 'series' && parts.length === 4 && parts[2] === 'episodios') {
        const series = this.contentRoutes.parseRef(parts[1]);
        const episode = this.contentRoutes.parseEpisodeRef(parts[3]);
        sourceMatches = await this.contentRoutes.origin(sourceId) === parsed.queryParamMap.get('o');
        kind = 'episode'; itemId = episode?.id || ''; seriesId = series?.id;
      } else if (parts[0] === 'sources') {
        sourceMatches = parts[1] === sourceId;
        if (parts[2] === 'live' && parts.length === 4) { kind = 'live'; itemId = parts[3]; }
        else if (parts[2] === 'movies' && parts.length === 5 && parts[4] === 'watch') { kind = 'movie'; itemId = parts[3]; }
        else if (parts[2] === 'series' && parts.length === 6 && parts[4] === 'episodes') { kind = 'episode'; seriesId = parts[3]; itemId = parts[5]; }
      }
    } catch { return false; }
    return sourceMatches && kind === currentKind && itemId === currentItemId && seriesId === currentSeriesId;
  }

  private async closeActivePlayback(): Promise<void> {
    if (this.closeOperation) return this.closeOperation;
    const task = (async () => {
      this.closing = true;
      const playback = this.playback();
      this.stopPlaybackTimers();
      const shouldCancelTranscode = this.transcodeRequested;
      const transcode = this.transcodeTask;
      const firstTranscodeCancel = playback && shouldCancelTranscode
        ? firstValueFrom(this.api.cancelTranscode(playback.id)).catch(() => undefined)
        : Promise.resolve(undefined);
      if (playback && this.playbackState.kind() !== 'live') {
        await this.progressSave.catch(() => undefined);
        await this.saveProgress(true);
      }
      this.detachMedia();
      await firstTranscodeCancel;
      if (transcode) await transcode.catch(() => undefined);
      if (playback && shouldCancelTranscode) await firstValueFrom(this.api.cancelTranscode(playback.id)).catch(() => undefined);
      if (playback) await this.cleanupPlayback(playback, false);
      if (playback && this.playbackState.playback()?.id === playback.id) this.playbackState.clear();
      this.sourcePlayback = undefined;
      this.activeSeason = undefined;
      this.transcoding.set(false); this.transcodeBusy.set(false); this.transcodeRequested = false;
      this.activeManifest = undefined; this.transcodeGeneration = ''; this.pausedAt = undefined; this.transcodeTask = undefined;
    })();
    this.closeOperation = task;
    try { await task; } finally { if (this.closeOperation === task) this.closeOperation = undefined; }
  }

  private async cleanupPlayback(playback: Playback, cancelTranscode = false): Promise<void> {
    const existing = this.closeOperations.get(playback.id);
    if (existing) return existing;
    const cleanup: Promise<unknown>[] = [firstValueFrom(this.api.closePlayback(playback.id)).catch(() => undefined)];
    if (cancelTranscode) cleanup.push(firstValueFrom(this.api.cancelTranscode(playback.id)).catch(() => undefined));
    const operation = Promise.all(cleanup).then(() => undefined);
    this.closeOperations.set(playback.id, operation);
    return operation;
  }

  private async loadMedia(url: string, format: 'hls'|'file', playbackId: string, autoplay = true): Promise<void> {
    if (this.closing || this.playbackState.playback()?.id !== playbackId) return;
    const attempt = ++this.mediaAttempt;
    const video = this.videoRef?.nativeElement;
    if (!video) { queueMicrotask(() => { if (attempt === this.mediaAttempt && !this.closing && this.playbackState.playback()?.id === playbackId) void this.loadMedia(url, format, playbackId); }); return; }
    this.hls?.destroy(); this.hls = undefined; video.pause(); video.removeAttribute('src'); video.load();
    this.failed.set(false); this.loading.set(true);
    if (format === 'hls') {
      try {
        const { default: Hls } = await import('hls.js');
        if (attempt !== this.mediaAttempt || this.closing || this.playbackState.playback()?.id !== playbackId) return;
        if (Hls.isSupported()) {
          const hls = new Hls({ enableWorker: true, lowLatencyMode: true });
          this.hls = hls;
          hls.on(Hls.Events.ERROR, (_event, raw) => {
            const data = raw as { fatal?: boolean; type?: string };
            if (data?.fatal) {
              const detail = data.type === 'mediaError'
                ? 'El navegador no pudo decodificar el flujo HLS. El códec concreto no está confirmado.'
                : 'Falló la carga del flujo HLS. Consultando el diagnóstico de transporte disponible.';
              this.failAttempt(attempt, 'No se pudo reproducir el flujo HLS', detail, data.type === 'mediaError', data.type !== 'mediaError');
            }
          });
          const updateAudio = () => {
            if (this.transcoding() && this.trackRevision) {
              this.audioChoices.set(this.inspectedTracks.filter(track => track.type === 'audio').map(track => ({key: `server:${track.id}`, label: `${track.label}${track.selectable ? '' : ' · No disponible'}`, disabled: !track.selectable})));
              return;
            }
            const choices = hls.audioTracks.map((track, index) => ({ key: `hls-audio:${index}`, label: this.trackLabel(track.lang, track.name, index) }));
            this.audioChoices.set(choices);
            const selected = hls.audioTrack;
            this.selectedAudio.set(selected >= 0 ? `hls-audio:${selected}` : '');
          };
          const updateSubtitles = () => {
            if (this.transcoding() && this.trackRevision) {
              this.subtitleChoices.set(this.inspectedTracks.filter(track => track.type === 'subtitle').map(track => ({key: `server:${track.id}`, label: `${track.label}${track.selectable ? '' : ` · ${track.unavailable === 'subtitle_not_text' ? 'Formato de imagen' : 'No disponible'}`}`, disabled: !track.selectable})));
              return;
            }
            const choices = hls.subtitleTracks.map((track, index) => ({ key: `hls-subtitle:${index}`, label: this.trackLabel(track.lang, track.name, index) }));
            this.subtitleChoices.set(choices);
            const selected = hls.subtitleTrack;
            this.selectedSubtitle.set(selected >= 0 ? `hls-subtitle:${selected}` : 'off');
          };
          hls.on(Hls.Events.AUDIO_TRACKS_UPDATED, updateAudio);
          hls.on(Hls.Events.AUDIO_TRACK_SWITCHED, updateAudio);
          hls.on(Hls.Events.SUBTITLE_TRACKS_UPDATED, updateSubtitles);
          hls.on(Hls.Events.SUBTITLE_TRACK_SWITCH, updateSubtitles);
          hls.loadSource(url); hls.attachMedia(video);
          hls.on(Hls.Events.MANIFEST_PARSED, () => {
            if (attempt === this.mediaAttempt && !this.closing) { if (autoplay) void this.play(); else this.loading.set(false); }
          });
          return;
        }
        if (video.canPlayType('application/vnd.apple.mpegurl')) {
          video.src = url;
          video.addEventListener('loadedmetadata', this.nativeTrackChange);
          video.addEventListener('loadeddata', this.nativeTrackChange);
          this.refreshNativeTracks();
          if (autoplay) void this.play();
          return;
        }
        this.failureTitle.set('Este navegador no admite HLS');
        this.failureDetail.set('Prueba Safari en iPhone o iPad, que incluye soporte HLS nativo.');
        this.failed.set(true); this.loading.set(false);
      } catch {
        if (attempt !== this.mediaAttempt || this.closing || this.playbackState.playback()?.id !== playbackId) return;
        this.failureTitle.set('No se pudo preparar el reproductor HLS');
        this.failureDetail.set('Recarga la página o prueba otro navegador.');
        this.failed.set(true); this.loading.set(false);
      }
    } else {
      video.src = url;
      video.addEventListener('loadedmetadata', this.nativeTrackChange);
      video.addEventListener('loadeddata', this.nativeTrackChange);
      this.refreshNativeTracks();
      if (autoplay) void this.play();
    }
  }

  private trackLabel(language: string | undefined, name: string | undefined, index: number): string {
    const label = (name || language || '').replace(/[\u0000-\u001f\u007f]/g, '').trim().slice(0, 96);
    return label || `Pista ${index + 1}`;
  }

  private refreshNativeTracks(): void {
    const video = this.videoRef?.nativeElement as (HTMLVideoElement & { audioTracks?: NativeAudioTrackList }) | undefined;
    if (!video || this.hls) return;
    const audios = video.audioTracks;
    if (audios !== this.nativeAudioList) {
      this.nativeAudioList?.removeEventListener('change', this.nativeTrackChange);
      this.nativeAudioList = audios;
      this.nativeAudioList?.addEventListener('change', this.nativeTrackChange);
    }
    const audioChoices = audios ? Array.from({length: audios.length}, (_, index) => ({key: `native-audio:${index}`, label: this.trackLabel(audios[index]?.language, audios[index]?.label, index)})) : [];
    const selectedAudio = audios ? Array.from({length: audios.length}, (_, index) => index).find(index => audios[index]?.enabled) : undefined;
    const texts = video.textTracks;
    if (texts !== this.nativeTextList) {
      this.nativeTextList?.removeEventListener('change', this.nativeTrackChange);
      this.nativeTextList = texts;
      this.nativeTextList.addEventListener('change', this.nativeTrackChange);
      this.nativeTextList.addEventListener('addtrack', this.nativeTrackChange);
      this.nativeTextList.addEventListener('removetrack', this.nativeTrackChange);
    }
    const available: Array<{key: string; label: string}> = [];
    for (let i = 0; i < texts.length; i++) {
      const track = texts[i];
      if (track.kind !== 'subtitles' && track.kind !== 'captions') continue;
      available.push({key: `native-subtitle:${i}`, label: this.trackLabel(track.language, track.label, available.length)});
    }
    const selected = Array.from({length: texts.length}, (_, index) => index).find(index => (texts[index].kind === 'subtitles' || texts[index].kind === 'captions') && texts[index].mode === 'showing');
    const serverAudios = this.inspectedTracks.filter(track => track.type === 'audio').map(track => ({
      key: `server:${track.id}`,
      label: `${track.label}${track.selectable ? '' : ` · ${track.unavailable === 'audio_codec_unavailable' ? 'Códec no compatible' : 'No disponible'}`}`,
      disabled: !track.selectable,
    }));
    const serverSubtitles = this.inspectedTracks.filter(track => track.type === 'subtitle').map(track => ({
      key: `server:${track.id}`,
      label: `${track.label}${track.selectable ? '' : ` · ${track.unavailable === 'subtitle_not_text' ? 'Formato de imagen' : 'No disponible'}`}`,
      disabled: !track.selectable,
    }));
    const useNativeAudio = audioChoices.length > 1;
    const useNativeSubtitles = available.length > 0;
    this.audioChoices.set(useNativeAudio || serverAudios.length === 0 ? audioChoices : serverAudios);
    this.subtitleChoices.set(useNativeSubtitles || serverSubtitles.length === 0 ? available : serverSubtitles);
    if (useNativeAudio) this.selectedAudio.set(selectedAudio === undefined ? '' : `native-audio:${selectedAudio}`);
    else if (serverAudios.length && !this.selectedAudio().startsWith('server:')) this.selectedAudio.set('');
    if (useNativeSubtitles) this.selectedSubtitle.set(selected === undefined ? 'off' : `native-subtitle:${selected}`);
    else if (serverSubtitles.length && !this.selectedSubtitle().startsWith('server:')) this.selectedSubtitle.set('off');
    else if (serverSubtitles.length === 0) this.selectedSubtitle.set(selected === undefined ? 'off' : `native-subtitle:${selected}`);
  }

  async inspectTracks(): Promise<void> {
    const playback = this.playback();
    if (!playback || this.tracksBusy()) return;
    const video = this.videoRef?.nativeElement;
    const wasPlaying = !!video && !video.paused;
    const position = this.logicalPosition();
    if (wasPlaying) video?.pause();
    this.tracksBusy.set(true);
    this.tracksMessage.set('Inspeccionando el archivo con límites de lectura.');
    try {
      const response = await firstValueFrom(this.api.playbackTracks(playback.id));
      if (this.closing || this.playback()?.id !== playback.id) return;
      this.inspectedTracks = response.tracks.filter(track => /^[A-Za-z0-9_-]{20,40}$/.test(track.id));
      this.trackRevision = response.revision || '';
      this.audioChoices.set(this.inspectedTracks.filter(track => track.type === 'audio').map(track => ({key: `server:${track.id}`, label: `${track.label}${track.selectable ? '' : ` · ${track.unavailable === 'audio_codec_unavailable' ? 'Códec no compatible' : 'No disponible'}`}`, disabled: !track.selectable})));
      this.subtitleChoices.set(this.inspectedTracks.filter(track => track.type === 'subtitle').map(track => ({key: `server:${track.id}`, label: `${track.label}${track.selectable ? '' : ` · ${track.unavailable === 'subtitle_not_text' ? 'Formato de imagen' : 'No disponible'}`}`, disabled: !track.selectable})));
      this.tracksMessage.set(response.status === 'complete'
        ? (this.inspectedTracks.length ? 'Las pistas elegidas requieren preparar una versión compatible de hasta 720p.' : 'No se encontraron pistas seleccionables en el límite de inspección.')
        : 'La inspección fue parcial; no confirma que el archivo carezca de pistas.');
    } catch (error) {
      this.tracksMessage.set(error instanceof HttpErrorResponse && error.status === 503 ? 'La inspección está desactivada en esta instalación.' : 'No se pudo inspeccionar el archivo. La reproducción sigue disponible.');
    } finally {
      if (this.closing || this.playback()?.id !== playback.id || this.sourcePlayback?.id !== playback.id) return;
      this.tracksBusy.set(false);
      const directFormat = this.sourcePlayback.format;
      if (directFormat === 'file') {
        // The API drains the current upstream read before ffprobe. Reattach the
        // media URL so an interrupted range request cannot leave the player in
        // a network-error state, then let onMetadata restore the saved offset.
        this.playbackState.resumePosition.set(position);
        await this.loadMedia(playback.url, directFormat, playback.id, false);
        if (wasPlaying && !this.closing && this.playback()?.id === playback.id) await this.play();
      }
    }
  }

  async chooseAudio(event: Event): Promise<void> {
    const select = event.target as HTMLSelectElement;
    const key = select.value;
    if (key.startsWith('server:')) {
      if (!window.confirm('Preparar una versión compatible de hasta 720p para usar esta pista de audio?')) { this.refreshTrackSelection(); select.value = this.selectedAudio(); return; }
      const track = this.inspectedTracks.find(value => `server:${value.id}` === key);
      if (track) {
        const currentSub = this.selectedSubtitle().startsWith('server:') ? this.selectedSubtitle().slice(7) : undefined;
        this.startTranscode(true, this.logicalPosition(), {trackRevision: this.trackRevision, audioTrackId: track.id, ...(currentSub ? {subtitleTrackId: currentSub} : {})});
      }
      return;
    }
    if (key.startsWith('hls-audio:') && this.hls) { this.hls.audioTrack = Number(key.slice(10)); return; }
    else if (key.startsWith('native-audio:')) {
      const tracks = (this.videoRef?.nativeElement as (HTMLVideoElement & {audioTracks?: NativeAudioTrackList}) | undefined)?.audioTracks;
      const selected = Number(key.slice(13));
      if (tracks) for (let i = 0; i < tracks.length; i++) tracks[i].enabled = i === selected;
    }
    this.refreshTrackSelection();
  }

  async chooseSubtitle(event: Event): Promise<void> {
    const select = event.target as HTMLSelectElement;
    const key = select.value;
    if (key.startsWith('server:')) {
      if (!window.confirm('Preparar una versión compatible de hasta 720p para mostrar esta pista de subtítulos?')) { this.refreshTrackSelection(); select.value = this.selectedSubtitle(); return; }
      const track = this.inspectedTracks.find(value => `server:${value.id}` === key);
      if (track) {
        const currentAudio = this.selectedAudio().startsWith('server:') ? this.selectedAudio().slice(7) : undefined;
        this.startTranscode(true, this.logicalPosition(), {trackRevision: this.trackRevision, subtitleTrackId: track.id, ...(currentAudio ? {audioTrackId: currentAudio} : {})});
      }
      return;
    }
    if (key === 'off' && this.selectedSubtitle().startsWith('server:')) {
      this.transcodeSelection = {...this.transcodeSelection, subtitleTrackId: undefined};
      this.selectedSubtitle.set('off');
      this.clearSubtitleCues();
      return;
    }
    if (this.hls) {
      this.hls.subtitleTrack = key.startsWith('hls-subtitle:') ? Number(key.slice(13)) : -1;
      this.hls.subtitleDisplay = key !== 'off';
      return;
    } else {
      const texts = this.videoRef?.nativeElement.textTracks;
      if (texts) for (let i = 0; i < texts.length; i++) if (texts[i].kind === 'subtitles' || texts[i].kind === 'captions') texts[i].mode = key === `native-subtitle:${i}` ? 'showing' : 'disabled';
    }
    this.selectedSubtitle.set(key || 'off');
  }

  private refreshTrackSelection(): void {
    if (this.transcoding() && this.trackRevision) {
      this.audioChoices.set(this.inspectedTracks.filter(track => track.type === 'audio').map(track => ({
        key: `server:${track.id}`,
        label: `${track.label}${track.selectable ? '' : ` · ${track.unavailable === 'audio_codec_unavailable' ? 'Códec no compatible' : 'No disponible'}`}`,
        disabled: !track.selectable,
      })));
      this.subtitleChoices.set(this.inspectedTracks.filter(track => track.type === 'subtitle').map(track => ({
        key: `server:${track.id}`,
        label: `${track.label}${track.selectable ? '' : ` · ${track.unavailable === 'subtitle_not_text' ? 'Formato de imagen' : 'No disponible'}`}`,
        disabled: !track.selectable,
      })));
      return;
    }
    if (this.hls) {
      this.selectedAudio.set(this.hls.audioTrack >= 0 ? `hls-audio:${this.hls.audioTrack}` : '');
      this.selectedSubtitle.set(this.hls.subtitleTrack >= 0 ? `hls-subtitle:${this.hls.subtitleTrack}` : 'off');
    } else this.refreshNativeTracks();
  }

  private startSubtitlePolling(): void {
    if (!this.transcoding() || !this.isPlaying() || !this.transcodeGeneration || !this.selectedSubtitle().startsWith('server:') || this.subtitlePoll) return;
    void this.fetchSubtitleCues();
    this.subtitlePoll = setInterval(() => void this.fetchSubtitleCues(), 2000);
  }

  private stopSubtitlePolling(): void {
    if (this.subtitlePoll) clearInterval(this.subtitlePoll);
    this.subtitlePoll = undefined;
  }

  private clearSubtitleCues(): void {
    this.cueAttempt++;
    if (this.cueTrack) {
      for (let i = this.cueTrack.cues?.length ?? 0; i > 0; i--) this.cueTrack.removeCue(this.cueTrack.cues![i - 1]);
      this.cueTrack.mode = 'disabled';
    }
    this.stopSubtitlePolling();
  }

  private async fetchSubtitleCues(): Promise<void> {
    const playback = this.playback();
    const generation = this.transcodeGeneration;
    if (!playback || !generation || !this.selectedSubtitle().startsWith('server:') || !this.isPlaying()) return;
    const attempt = this.cueAttempt;
    const position = this.logicalPosition();
    const endpoint = `/api/v1/playbacks/${encodeURIComponent(playback.id)}/transcode/${encodeURIComponent(generation)}/subtitles.vtt?at=${encodeURIComponent(String(position))}`;
    try {
      const response = await fetch(endpoint, {credentials: 'same-origin', cache: 'no-store', headers: {'Accept': 'text/vtt'}});
      if (!response.ok) return;
      const vtt = await response.text();
      if (attempt !== this.cueAttempt || this.closing || this.playback()?.id !== playback.id || generation !== this.transcodeGeneration) return;
      this.applyVODCues(vtt);
    } catch { /* a missed polling window is retried on the next tick */ }
  }

  private applyVODCues(vtt: string): void {
    const video = this.videoRef?.nativeElement;
    if (!video || !vtt.startsWith('WEBVTT') || vtt.length > 2 * 1024 * 1024) return;
    if (!this.cueTrack) this.cueTrack = video.addTextTrack('subtitles', 'Subtítulos');
    const track = this.cueTrack;
    for (let i = track.cues?.length ?? 0; i > 0; i--) track.removeCue(track.cues![i - 1]);
    const blocks = vtt.split(/\r?\n\s*\r?\n/).slice(1, 513);
    let bytes = 0;
    for (const block of blocks) {
      const lines = block.split(/\r?\n/);
      const timeIndex = lines.findIndex(line => line.includes('-->'));
      if (timeIndex < 0) continue;
      const [startText, endTextRaw] = lines[timeIndex].split('-->');
      const start = this.parseCueTime(startText) - this.transcodeOffset();
      const end = this.parseCueTime(endTextRaw.trim().split(/\s+/)[0]) - this.transcodeOffset();
      const text = lines.slice(timeIndex + 1).join('\n').replace(/&lt;/g, '<').replace(/&gt;/g, '>').replace(/&amp;/g, '&');
      bytes += text.length;
      if (!Number.isFinite(start) || !Number.isFinite(end) || start < 0 || end <= start || !text || text.length > 16 * 1024 || bytes > 2 * 1024 * 1024) continue;
      try { track.addCue(new VTTCue(start, end, text)); } catch { /* skip malformed cue */ }
    }
    track.mode = this.selectedSubtitle().startsWith('server:') ? 'showing' : 'disabled';
  }

  private parseCueTime(value: string): number {
    const match = /^(?:(\d{2,}):)?(\d{2}):(\d{2})\.(\d{3})$/.exec(value.trim());
    if (!match) return NaN;
    return Number(match[1] || 0) * 3600 + Number(match[2]) * 60 + Number(match[3]) + Number(match[4]) / 1000;
  }

  private async performTranscode(offsetSeconds: number, selection?: {trackRevision?: string; audioTrackId?: string; subtitleTrackId?: string}): Promise<void> {
    const playback = this.playback();
    if (!playback || this.transcodeBusy() || this.closing || this.playbackState.kind() === 'live' || !Number.isFinite(offsetSeconds) || offsetSeconds < 0) return;
    const requestedOffset = Math.floor(offsetSeconds);
    const replacingTranscode = this.transcoding();
    this.transcodeResumeAfterStart = !this.videoRef?.nativeElement.paused;
    if (selection) {
      this.transcodeSelection = selection;
      this.clearSubtitleCues();
      this.selectedSubtitle.set('off');
    }
    this.pausedAt = undefined;
    this.transcodeBusy.set(true);
    this.failed.set(false);
    this.needsTap.set(false);
    this.allowExplicitTranscode.set(false);
    this.loading.set(true);
    this.failureTitle.set('No se pudo preparar la versión compatible');
    this.failureDetail.set('Prueba de nuevo más tarde o selecciona otra versión.');
    if (!this.transcoding()) {
      this.transcodeOffset.set(requestedOffset);
      this.transcodeDuration.set(0);
    } else {
      this.transcodeOffset.set(requestedOffset);
    }
    this.playbackState.resumePosition.set(requestedOffset);
    this.transcodeRequested = true;
    this.activeManifest = undefined;
    this.shownPosition.set(requestedOffset);
    this.detachMedia();
    try {
      const result = await firstValueFrom(this.api.transcodePlayback(playback.id, requestedOffset, this.transcodeSelection));
      if (this.closing || this.playbackState.playback()?.id !== playback.id) return;
      if (result.playbackId !== playback.id || typeof result.generation !== 'string' || !result.generation.trim() || !Number.isFinite(result.offsetSeconds) || result.offsetSeconds < 0 || !Number.isFinite(result.durationSeconds) || result.durationSeconds < 0 || typeof result.manifestUrl !== 'string') {
        throw new Error('Respuesta de conversión no válida');
      }
      const manifest = new URL(result.manifestUrl, window.location.origin);
      if (manifest.origin !== window.location.origin) throw new Error('El manifiesto debe proceder de la API local');
      this.transcodeOffset.set(result.offsetSeconds);
      this.transcodeDuration.set(result.durationSeconds);
      this.transcodeGeneration = result.generation;
      if (result.audioTrackId) this.selectedAudio.set(`server:${result.audioTrackId}`);
      else if (this.selectedAudio().startsWith('server:')) this.selectedAudio.set('');
      if (result.subtitleTrackId) this.selectedSubtitle.set(`server:${result.subtitleTrackId}`);
      else this.selectedSubtitle.set('off');
      this.shownPosition.set(result.offsetSeconds);
      this.playbackState.resumePosition.set(0);
      this.activeManifest = manifest.href;
      this.transcoding.set(true);
      await this.loadMedia(manifest.href, 'hls', playback.id, this.transcodeResumeAfterStart);
      this.startSubtitlePolling();
    } catch (error) {
      if (this.closing) return;
      let vodDiagnostic: { cause: string; upstreamStatus?: number } | undefined;
      if (error instanceof HttpErrorResponse && error.error?.error?.code === 'TRANSCODER_FAILED') {
        try { vodDiagnostic = await firstValueFrom(this.api.playbackDiagnostic(playback.id)); } catch { /* keep the conversion error generic if diagnostics are unavailable */ }
        if (this.closing) return;
      }
      if (this.transcodeRequested) {
        try { await firstValueFrom(this.api.cancelTranscode(playback.id)); } catch { /* failed startup may already have released the generation */ }
      }
      if (this.closing) return;
      this.failed.set(true);
      this.loading.set(false);
      this.transcoding.set(false);
      this.failureTitle.set('No se pudo preparar la versión compatible');
      if (error instanceof HttpErrorResponse && error.status === 409) {
        const code = error.error?.error?.code;
        this.failureDetail.set(code === 'PLAYBACK_LIMIT'
          ? 'La cuenta IPTV ya está usando su conexión. Cierra la otra reproducción e inténtalo de nuevo.'
          : 'El conversor ya está ocupado o esta reproducción no puede convertirse ahora. Prueba otra versión.');
      } else if (error instanceof HttpErrorResponse && (error.status === 401 || error.status === 403)) {
        this.failureDetail.set('La sesión de reproducción ha caducado. Vuelve al catálogo e inicia el contenido otra vez.');
      } else if (error instanceof HttpErrorResponse && (error.status === 404 || error.status === 501)) {
        this.failureDetail.set('La conversión no está disponible en esta instalación. Prueba una versión MP4 o H.264.');
      } else if (error instanceof HttpErrorResponse && error.status === 503 && error.error?.error?.code === 'TRANSCODING_DISABLED') {
        this.failureDetail.set('La conversión de vídeo está desactivada en esta instalación. Prueba una versión MP4 o H.264.');
      } else if (error instanceof HttpErrorResponse && error.error?.error?.code === 'TRANSCODER_FAILED') {
        this.failureDetail.set(this.vodTranscodeDiagnosticMessage(vodDiagnostic) ?? 'No se pudo completar la conversión del flujo. El problema puede estar en el origen o en la conversión. Prueba otra versión.');
      } else {
        this.playbackState.resumePosition.set(requestedOffset);
        this.failureDetail.set('El conversor no pudo preparar este archivo. Prueba otra versión o inténtalo más tarde.');
      }
      const disabled = error instanceof HttpErrorResponse && error.status === 503 && error.error?.error?.code === 'TRANSCODING_DISABLED';
      if (replacingTranscode && !disabled && !(error instanceof HttpErrorResponse && [401, 403, 404, 409, 501].includes(error.status))) this.allowExplicitTranscode.set(true);
    } finally {
      this.transcodeBusy.set(false);
    }
  }

  private detachMedia(): void {
    this.mediaAttempt++;
    this.clearSubtitleCues();
    this.nativeAudioList?.removeEventListener('change', this.nativeTrackChange);
    this.nativeAudioList = undefined;
    this.nativeTextList?.removeEventListener('change', this.nativeTrackChange);
    this.nativeTextList?.removeEventListener('addtrack', this.nativeTrackChange);
    this.nativeTextList?.removeEventListener('removetrack', this.nativeTrackChange);
    this.nativeTextList = undefined;
    this.hls?.destroy(); this.hls = undefined;
    const video = this.videoRef?.nativeElement;
    if (video) { video.removeEventListener('loadedmetadata', this.nativeTrackChange); video.removeEventListener('loadeddata', this.nativeTrackChange); video.pause(); video.removeAttribute('src'); video.load(); }
  }

  private async initializePlayback(playback: Playback): Promise<void> {
    if (!this.closing && this.playbackState.playback()?.id === playback.id) await this.loadMedia(playback.url, playback.format, playback.id);
  }

  private saveProgress(final = false): Promise<void> {
    const playback = this.playback(), video = this.videoRef?.nativeElement;
    if (!playback || !video || this.playbackState.kind() === 'live' || !Number.isFinite(video.currentTime)) return Promise.resolve();
    if (final && this.saved) return Promise.resolve();
    if (final) this.saved = true;
    if (this.transcoding()) {
      const position = this.logicalPosition();
      return firstValueFrom(this.api.saveProgress(playback.contentKey, playback.title, this.playbackState.kind(), position, this.transcodeDuration())).then(() => undefined, () => undefined);
    }
    if (this.playbackState.resumePosition() > 0) {
      return firstValueFrom(this.api.saveProgress(playback.contentKey, playback.title, this.playbackState.kind(), this.playbackState.resumePosition(), 0)).then(() => undefined, () => undefined);
    }
    if (!Number.isFinite(video.duration) || video.duration <= 0) return Promise.resolve();
    return firstValueFrom(this.api.saveProgress(playback.contentKey, playback.title, this.playbackState.kind(), Math.max(0, video.currentTime), video.duration)).then(() => undefined, () => undefined);
  }

  private failSession(): void {
    this.failureTitle.set('La sesión de reproducción ha caducado');
    this.failureDetail.set('Vuelve al catálogo para iniciar el contenido otra vez.');
    this.failed.set(true); this.loading.set(false); this.releaseMedia();
  }
  private failAttempt(attempt: number, title: string, fallback: string, formatFailure = false, transportFailure = false): void {
    if (attempt !== this.mediaAttempt || this.failed() || this.closing) return;
    this.failureTitle.set(title);
    this.failureDetail.set(fallback);
    this.allowExplicitTranscode.set(false);
    this.failed.set(true); this.loading.set(false);
    const playback = this.playback();
    if (!playback || this.diagnosticRequestedAttempt === attempt) return;
    this.diagnosticRequestedAttempt = attempt;
    this.api.playbackDiagnostic(playback.id).subscribe({
      next: diagnostic => {
        if (attempt !== this.mediaAttempt || this.closing || !this.failed()) return;
        const message = this.diagnosticMessage(diagnostic.cause, diagnostic.upstreamStatus);
        const cause = diagnostic.cause.trim().toLowerCase().replace(/[\s-]+/g, '_');
        if (message) this.failureDetail.set(message);
        if (cause === 'none' && formatFailure && !transportFailure && this.playbackState.kind() !== 'live' && playback.format === 'file' && !this.automaticFallbackAttempted) {
          this.startTranscode(false);
          return;
        }
        if (!message && !transportFailure && this.playbackState.kind() !== 'live' && playback.format === 'file') {
          this.allowExplicitTranscode.set(true);
          this.failureDetail.set('No se pudo confirmar si el problema es de formato. Puedes probar una versión compatible de hasta 720p.');
        }
      },
      error: error => {
        if (attempt !== this.mediaAttempt || this.closing || !this.failed()) return;
        if (error instanceof HttpErrorResponse && (error.status === 401 || error.status === 403)) {
          this.failureTitle.set('La sesión de reproducción ha caducado');
          this.failureDetail.set('Vuelve al catálogo e inicia el contenido otra vez.');
        } else if (error instanceof HttpErrorResponse && error.status === 409) {
          this.failureTitle.set('La cuenta IPTV está ocupada');
          this.failureDetail.set('La cuenta ya tiene una reproducción activa. Cierra la otra reproducción antes de continuar.');
        } else if (!transportFailure && this.playbackState.kind() !== 'live' && playback.format === 'file') {
          this.allowExplicitTranscode.set(true);
          this.failureDetail.set('No se pudo confirmar la causa del fallo. Puedes probar una versión compatible de hasta 720p.');
        }
      },
    });
  }
  private handleHeartbeatError(error: unknown): void {
    if (this.isPlaybackLimit(error)) {
      this.showPlaybackLimit();
      return;
    }
    this.failSession();
  }
  private isPlaybackLimit(error: unknown): boolean {
    return error instanceof HttpErrorResponse && error.status === 409 && error.error?.error?.code === 'PLAYBACK_LIMIT';
  }
  private showPlaybackLimit(): void {
    this.failureTitle.set('La cuenta IPTV está ocupada');
    this.failureDetail.set('La cuenta ya tiene una reproducción activa. Espera a que termine antes de volver a intentarlo.');
    this.failed.set(true); this.loading.set(false); this.releaseMedia();
  }
  private diagnosticMessage(cause: string, status?: number): string | undefined {
    const normalized = cause.toLowerCase().replace(/[\s-]+/g, '_');
    if (!normalized || normalized === 'none' || normalized === 'unknown') return undefined;
    if (normalized.includes('upstream') || normalized.includes('status')) {
      if (status === 511) return 'Un proxy o gateway intermedio solicita autenticación de red (HTTP 511). Revisa la salida a Internet del servidor de la API o su proxy.';
      if (status === 401 || status === 403) return `El servidor del contenido rechazó la solicitud (HTTP ${status}). Comprueba que el enlace siga vigente y que tu cuenta IPTV permita reproducirlo.`;
      if (status === 404 || status === 410) return `El servidor no encontró el archivo o el enlace ha caducado (HTTP ${status}). Prueba otra versión del contenido.`;
      if (status !== undefined) return `El servidor del contenido respondió con HTTP ${status}. Prueba otra versión o inténtalo más tarde.`;
      return 'El servidor del contenido rechazó o no pudo atender la solicitud. Prueba otra versión.';
    }
    if (normalized.includes('timeout')) return 'El servidor tardó demasiado en responder. Comprueba tu conexión e inténtalo de nuevo.';
    if (normalized.includes('dns')) return 'No se pudo localizar el servidor o establecer una conexión con él. Puede ser un problema temporal de red o del enlace.';
    if (normalized.includes('connection') || normalized.includes('connect')) return 'No se pudo conectar con el servidor del contenido. Comprueba tu conexión o prueba otra versión.';
    if (normalized.includes('tls') || normalized.includes('certificate')) return 'No se pudo establecer una conexión segura con el servidor del contenido.';
    if (normalized.includes('destination') || normalized.includes('port') || normalized.includes('blocked') || normalized.includes('policy')) return 'El servidor usa un destino o puerto que la aplicación no puede abrir.';
    if (normalized.includes('redirect')) return 'El enlace redirigió demasiadas veces o a un destino no disponible.';
    if (normalized.includes('invalid_response') || normalized.includes('invalid')) return 'El servidor devolvió una respuesta de vídeo no válida o incompleta.';
    return undefined;
  }
  private vodTranscodeDiagnosticMessage(diagnostic?: { cause: string; upstreamStatus?: number }): string | undefined {
    if (!diagnostic) return undefined;
    const transportMessage = this.diagnosticMessage(diagnostic.cause, diagnostic.upstreamStatus);
    if (transportMessage) return transportMessage;
    const normalized = diagnostic.cause.toLowerCase().replace(/[\s-]+/g, '_');
    if (normalized === 'transcoder_failed' || normalized === 'unknown' || normalized === 'none' || !normalized) {
      return 'No se pudo completar la conversión del flujo. El problema puede estar en el origen o en la conversión. Prueba otra versión.';
    }
    return undefined;
  }
  private releaseMedia(): void {
    this.stopPlaybackTimers();
    this.detachMedia();
  }
  private stopPlaybackTimers(): void {
    if (this.heartbeat) clearInterval(this.heartbeat);
    if (this.progressTimer) clearInterval(this.progressTimer);
    this.heartbeat = undefined; this.progressTimer = undefined;
  }
  ngOnDestroy(): void { this.routeGeneration++; if (this.controlHideTimer) clearTimeout(this.controlHideTimer); this.clearSubtitleCues(); if (!this.closing) { this.saveProgress(true); this.releaseMedia(); } }
}
