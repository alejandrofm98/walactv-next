import { Component, DestroyRef, ElementRef, OnDestroy, OnInit, ViewChild, inject, signal } from '@angular/core';
import { HttpErrorResponse } from '@angular/common/http';
import { Router } from '@angular/router';
import { interval, firstValueFrom } from 'rxjs';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import type Hls from 'hls.js';
import { ApiService } from '../../core/api.service';
import { PlaybackStateService } from '../../core/playback-state.service';
import type { Playback } from '../../models';

@Component({
  standalone: true,
  template: `
    <main class="player-page" [class.live-player-page]="playbackState.kind() === 'live'" [class.vod-player-page]="playbackState.kind() !== 'live'">
      <header><div class="brand" aria-label="WalacTV"><span class="brand-mark">w<span>·</span></span><span>walac<span>tv</span></span></div></header>
      <section class="player-content">
        <button class="back-link" (click)="exit()">← <span>Volver al catálogo</span></button>
        <div class="player-title"><div>@if (playbackState.kind() !== 'live') { <p class="eyebrow"><span></span>{{ playbackState.kind() === 'movie' ? 'PELÍCULA' : 'EPISODIO' }}</p> }<h1>{{ playbackState.title() }}</h1></div>@if (playbackState.kind() === 'live') { <span class="live-badge"><i></i> En directo</span> }</div>
        <div #playerSurface class="playback-surface" [class.live-playback-surface]="playbackState.kind() === 'live'" [class.vod-playback-surface]="playbackState.kind() !== 'live'" (fullscreenchange)="syncFullscreen()">
        <div class="video-shell" [class.is-loading]="loading()">
          <video #video [controls]="!transcoding()" playsinline preload="metadata" (loadedmetadata)="onMetadata()" (timeupdate)="onTimeUpdate()" (waiting)="loading.set(true)" (playing)="playing()" (pause)="paused()" (volumechange)="syncVolume()" (ended)="onEnded()" (error)="onMediaError()"></video>
          @if (loading() && !failed()) { <div class="video-message loading-message"><span class="loader"></span><b>Preparando la reproducción</b></div> }
          @if (needsTap() && !failed()) { <button class="tap-to-play" (click)="play()" aria-label="Reproducir"><span aria-hidden="true">▶</span><b>Reproducir</b></button> }
          @if (failed()) { <div class="video-message failure-message"><span>!</span><b>{{ failureTitle() }}</b><small>{{ failureDetail() }}</small>@if (canOfferTranscode()) { <button (click)="startTranscode(true)" [disabled]="transcodeBusy()">{{ transcodeBusy() ? 'Preparando versión compatible…' : 'Probar versión compatible (hasta 720p)' }}</button> }<button class="secondary-action" (click)="retry()">Reintentar</button></div> }
        </div>
        @if (transcoding()) { <div class="transcode-status"><span>Versión compatible · hasta 720p</span>@if (transcodeDuration() > 0) { <label class="timeline"><span>{{ formatTime(logicalPosition()) }}</span><input aria-label="Buscar en la película" type="range" min="0" [max]="transcodeDuration()" step="1" [value]="Math.min(logicalPosition(), transcodeDuration())" [disabled]="transcodeBusy()" (change)="seekTranscode($event)"><span>{{ formatTime(transcodeDuration()) }}</span></label> } @else { <small>Duración desconocida. La barra de búsqueda no está disponible.</small> }<small class="pause-policy">Si pausas más de 45 segundos, al reanudar se prepara un flujo nuevo desde la posición pausada.</small><div class="transcode-controls" role="group" aria-label="Controles del reproductor"><button type="button" [attr.aria-label]="isPlaying() ? 'Pausar' : 'Reproducir'" [disabled]="failed()" (click)="togglePlayback()">{{ isPlaying() ? 'Pausar' : 'Reproducir' }}</button><button type="button" [attr.aria-label]="muted() ? 'Activar sonido' : 'Silenciar'" [attr.aria-pressed]="muted()" (click)="toggleMute()">{{ muted() ? 'Activar sonido' : 'Silenciar' }}</button><label class="volume-control"><span>Volumen</span><input aria-label="Volumen" type="range" min="0" max="1" step="0.05" [value]="volume()" (input)="setVolume($event)"></label><button type="button" [attr.aria-label]="fullscreen() ? 'Salir de pantalla completa' : 'Pantalla completa'" [attr.aria-pressed]="fullscreen()" (click)="toggleFullscreen()">{{ fullscreen() ? 'Salir de pantalla completa' : 'Pantalla completa' }}</button></div></div> }
        </div>
        @if (playbackState.kind() !== 'live') { <div class="player-details"><div><span>FORMATO ORIGINAL</span><b>{{ playback()?.format === 'hls' ? 'HLS adaptable' : extension() }}</b></div><p>La compatibilidad depende del navegador y de sus códecs de vídeo.</p></div> }
      </section>
    </main>
  `,
  styles: [`
    :host{display:block;min-height:100vh;background:#11120f;color:#f0eee8}.player-page{min-height:100vh;background:radial-gradient(ellipse at 52% 36%,#27291f 0,transparent 48%),#11120f}.player-page>header{height:62px;border-bottom:1px solid #292b24;padding:0 clamp(20px,5vw,70px);display:flex;align-items:center;justify-content:space-between}.brand{display:flex;align-items:center;gap:10px;text-decoration:none;color:#f2f0e8;font-size:16px;font-weight:700;letter-spacing:-.7px}.brand>span:last-child span{color:#c6f268}.brand-mark{width:29px;height:29px;display:grid;place-content:center;border-radius:9px;background:#c6f268;color:#171913;font-weight:800;font-size:19px;letter-spacing:-2px}.brand-mark span{color:#657a37}.session-state{font-size:8px;letter-spacing:1.2px;color:#85877c}.session-state i{display:inline-block;width:5px;height:5px;background:#c6f268;border-radius:50%;margin-right:7px}.player-content{width:min(1060px,calc(100% - 42px));margin:0 auto;padding:35px 0 55px}.back-link{display:flex;gap:9px;align-items:center;background:none;border:0;color:#b3b5aa;font:inherit;font-size:10px;cursor:pointer}.back-link:hover{color:#c6f268}.player-title{display:flex;align-items:end;justify-content:space-between;margin:28px 0 17px}.eyebrow{font-size:8px;letter-spacing:1.4px;color:#b6c784;display:flex;align-items:center;gap:8px;margin:0}.eyebrow span{width:6px;height:6px;border-radius:50%;background:#f27a66;box-shadow:0 0 8px #f27a66}.player-title h1{font:clamp(23px,3.5vw,37px) Georgia,serif;letter-spacing:-.6px;font-weight:400;margin:10px 0 0;max-width:76vw}.stop-button{display:flex;align-items:center;gap:10px;padding:8px 12px;background:#24261f;border:1px solid #3a3d32;border-radius:5px;color:#c4c6bb;font:inherit;font-size:9px;cursor:pointer;white-space:nowrap}.stop-button span{font-size:17px;color:#c6f268}.video-shell{position:relative;aspect-ratio:16/9;max-height:67vh;background:#030403;border:1px solid #36392e;border-radius:8px;overflow:hidden;box-shadow:0 18px 75px #0007}.video-shell video{width:100%;height:100%;display:block;object-fit:contain;background:#030403}.video-shell.is-loading video{opacity:.2}.video-message,.tap-to-play{position:absolute;inset:0;display:flex;flex-direction:column;align-items:center;justify-content:center;text-align:center;background:#05060488;color:#edeee7}.video-message b,.tap-to-play b{font-size:13px;font-weight:550;margin-top:12px}.video-message small,.tap-to-play small{color:#9da095;font-size:10px;margin-top:7px}.loader{width:30px;height:30px;border:2px solid #747c5d;border-top-color:#c6f268;border-radius:50%;animation:spin .8s linear infinite}@keyframes spin{to{transform:rotate(360deg)}}.tap-to-play{border:0;background:#05060462;cursor:pointer;font:inherit}.tap-to-play span{width:56px;height:56px;border-radius:50%;background:#c6f268;color:#1a1d14;display:grid;place-items:center;padding-left:4px;font-size:17px;box-shadow:0 0 35px #c6f26855}.tap-to-play:hover span{transform:scale(1.07)}.failure-message>span{width:36px;height:36px;border-radius:50%;border:1px solid #c77f68;color:#ffb29a;display:grid;place-items:center;font-size:17px}.failure-message button{margin-top:17px;padding:9px 14px;border-radius:5px;background:#c6f268;color:#1a1c15;border:0;font-size:10px;font-weight:650;cursor:pointer}.failure-message button:disabled{opacity:.55;cursor:wait}.failure-message .secondary-action{margin-top:8px;background:#292c24;border:1px solid #45483b;color:#dedfd5}.transcode-status{display:flex;justify-content:space-between;align-items:center;gap:18px;color:#b9c993;font-size:10px;padding:10px 2px}.transcode-status small{color:#92958a}.transcode-status label{display:flex;align-items:center;gap:10px;min-width:min(460px,60%);color:#aeb0a5;font-size:9px}.transcode-status input{flex:1;accent-color:#c6f268;cursor:pointer}.video-corner{position:absolute;top:13px;left:15px;font-size:7px;letter-spacing:1.2px;color:#aeb3a1;background:#10120dbb;padding:6px 8px;border-radius:3px;pointer-events:none}.video-corner i{font-style:normal;color:#c6f268}.player-details{display:grid;grid-template-columns:1fr 1fr 1fr;gap:18px;padding:17px 4px;border-bottom:1px solid #303229}.player-details>div span,.player-details>div b{display:block}.player-details>div span{color:#777a6e;font-size:7px;letter-spacing:1px}.player-details>div b{margin-top:6px;font-size:10px;font-weight:500;color:#d1d2c9}.player-details>p{grid-column:1/-1;color:#777a70;font-size:9px;margin:2px 0 0}@media(max-width:600px){.player-content{width:calc(100% - 28px);padding-top:25px}.player-title{margin:22px 0 13px;align-items:center}.player-title h1{font-size:22px;line-height:1.2}.stop-button{font-size:0;padding:7px}.stop-button span{font-size:18px}.video-shell{aspect-ratio:16/10;max-height:none}.transcode-status{align-items:stretch;flex-direction:column;gap:8px}.transcode-status label{min-width:100%}.player-details{grid-template-columns:1fr 1fr;gap:15px}.player-details>div:last-of-type{display:none}.player-details>p{line-height:1.5}}
  `, `
    .transcode-controls{display:flex;align-items:center;justify-content:flex-end;gap:8px;width:100%;padding:9px 2px}.transcode-controls button{background:#24261f;border:1px solid #3a3d32;border-radius:5px;color:#e4e5dc;padding:8px 10px;font:inherit;font-size:10px;cursor:pointer}.transcode-controls button:hover:not(:disabled){border-color:#c6f268;color:#c6f268}.transcode-controls button:focus-visible,.transcode-controls input:focus-visible{outline:2px solid #c6f268;outline-offset:2px}.transcode-controls button:disabled{opacity:.5;cursor:not-allowed}.volume-control{display:flex;align-items:center;gap:7px;color:#aeb0a5;font-size:9px}.volume-control input{width:88px;accent-color:#c6f268}.timeline{flex:1}.pause-policy{color:#92958a}.playback-surface:fullscreen{display:flex;flex-direction:column;justify-content:center;background:#030403;padding:2vw}.playback-surface:fullscreen .video-shell{max-height:none;flex:1;aspect-ratio:16/9}.playback-surface:fullscreen .video-shell video{max-height:calc(100vh - 180px)}@media(max-width:600px){.transcode-controls{justify-content:space-between;flex-wrap:wrap}.transcode-controls button{padding:8px}.volume-control input{width:65px}.playback-surface:fullscreen{padding:8px}.playback-surface:fullscreen .video-shell{aspect-ratio:16/10}}
  `],
})
export class PlayerComponent implements OnInit, OnDestroy {
  @ViewChild('video') private videoRef?: ElementRef<HTMLVideoElement>;
  @ViewChild('playerSurface') private playerSurfaceRef?: ElementRef<HTMLElement>;
  readonly playbackState = inject(PlaybackStateService);
  private readonly api = inject(ApiService);
  private readonly router = inject(Router);
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
  private pausedAt?: number;
  private pausedPosition = 0;
  private readonly longPauseMs = 45_000;

  ngOnInit(): void {
    const playback = this.playbackState.playback();
    if (!playback) { void this.router.navigateByUrl('/'); return; }
    this.sourcePlayback = playback;
    void this.initializePlayback(playback.url, playback.format, playback.contentKey);
    if (this.playbackState.kind() !== 'live') {
      this.heartbeat = setInterval(() => this.api.heartbeatPlayback(playback.id).pipe(takeUntilDestroyed(this.destroyRef)).subscribe({ error: error => this.handleHeartbeatError(error) }), 10_000);
      this.progressTimer = setInterval(() => { this.progressSave = this.saveProgress(); }, 15_000);
    } else {
      this.heartbeat = setInterval(() => this.api.heartbeatPlayback(playback.id).pipe(takeUntilDestroyed(this.destroyRef)).subscribe({ error: error => this.handleHeartbeatError(error) }), 10_000);
    }
  }

  playback() { return this.playbackState.playback(); }
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
  }
  paused(): void {
    this.isPlaying.set(false);
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
    const video = this.videoRef?.nativeElement;
    if (!surface || !video) return;
    try {
      if (document.fullscreenElement) await document.exitFullscreen();
      else if (surface.requestFullscreen) await surface.requestFullscreen();
      else if (video.requestFullscreen) await video.requestFullscreen();
      this.syncFullscreen();
    } catch { /* fullscreen can be blocked by the browser or unavailable on this device */ }
  }

  syncFullscreen(): void {
    const surface = this.playerSurfaceRef?.nativeElement;
    this.fullscreen.set(!!surface && (document.fullscreenElement === surface || document.fullscreenElement === this.videoRef?.nativeElement));
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
    if (this.recoverLongPause()) return;
    try { await video.play(); this.needsTap.set(false); } catch { this.needsTap.set(true); }
  }

  retry(): void {
    if (this.transcoding()) {
      this.startTranscode(true);
      return;
    }
    this.failed.set(false); this.loading.set(true); this.needsTap.set(false);
    const playback = this.sourcePlayback;
    if (playback) this.loadMedia(playback.url, playback.format);
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

  startTranscode(explicit: boolean, offsetSeconds = this.logicalPosition()): void {
    if (this.transcodeBusy() || this.closing || (this.automaticFallbackAttempted && !explicit)) return;
    if (!explicit) this.automaticFallbackAttempted = true;
    const task = this.performTranscode(offsetSeconds);
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
    if (this.closing) return;
    this.closing = true;
    const pendingTranscode = this.transcodeTask;
    if (this.playbackState.kind() !== 'live') { await this.progressSave; await this.saveProgress(true); }
    this.releaseMedia();
    const playback = this.playback();
    if (playback) await this.cleanupPlayback(playback);
    if (pendingTranscode && playback) void pendingTranscode.then(() => this.cleanupPlayback(playback), () => this.cleanupPlayback(playback));
    this.playbackState.clear();
    await this.router.navigateByUrl('/');
  }

  async closeForNavigation(): Promise<boolean> {
    if (this.closing) return true;
    this.closing = true;
    const pendingTranscode = this.transcodeTask;
    if (this.playbackState.kind() !== 'live') { await this.progressSave; await this.saveProgress(true); }
    this.releaseMedia();
    const playback = this.playback();
    if (playback) await this.cleanupPlayback(playback);
    if (pendingTranscode && playback) void pendingTranscode.then(() => this.cleanupPlayback(playback), () => this.cleanupPlayback(playback));
    this.playbackState.clear();
    return true;
  }

  private async cleanupPlayback(playback: Playback): Promise<void> {
    const cleanup: Promise<unknown>[] = [firstValueFrom(this.api.closePlayback(playback.id)).catch(() => undefined)];
    if (this.transcodeRequested) cleanup.push(firstValueFrom(this.api.cancelTranscode(playback.id)).catch(() => undefined));
    await Promise.all(cleanup);
  }

  private async loadMedia(url: string, format: 'hls'|'file'): Promise<void> {
    const attempt = ++this.mediaAttempt;
    const video = this.videoRef?.nativeElement; if (!video) { queueMicrotask(() => void this.loadMedia(url, format)); return; }
    this.hls?.destroy(); this.hls = undefined; video.pause(); video.removeAttribute('src'); video.load();
    this.failed.set(false); this.loading.set(true);
    if (format === 'hls') {
      try {
        const { default: Hls } = await import('hls.js');
        if (attempt !== this.mediaAttempt || this.closing) return;
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
          hls.loadSource(url); hls.attachMedia(video);
          hls.on(Hls.Events.MANIFEST_PARSED, () => {
            if (attempt === this.mediaAttempt && !this.closing) void this.play();
          });
          return;
        }
        if (video.canPlayType('application/vnd.apple.mpegurl')) { video.src = url; void this.play(); return; }
        this.failureTitle.set('Este navegador no admite HLS');
        this.failureDetail.set('Prueba Safari en iPhone o iPad, que incluye soporte HLS nativo.');
        this.failed.set(true); this.loading.set(false);
      } catch {
        if (attempt !== this.mediaAttempt || this.closing) return;
        this.failureTitle.set('No se pudo preparar el reproductor HLS');
        this.failureDetail.set('Recarga la página o prueba otro navegador.');
        this.failed.set(true); this.loading.set(false);
      }
    } else {
      video.src = url;
      void this.play();
    }
  }

  private async performTranscode(offsetSeconds: number): Promise<void> {
    const playback = this.playback();
    if (!playback || this.transcodeBusy() || this.closing || this.playbackState.kind() === 'live' || !Number.isFinite(offsetSeconds) || offsetSeconds < 0) return;
    const requestedOffset = Math.floor(offsetSeconds);
    const replacingTranscode = this.transcoding();
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
      const result = await firstValueFrom(this.api.transcodePlayback(playback.id, requestedOffset));
      if (this.closing) {
        try { await firstValueFrom(this.api.cancelTranscode(playback.id)); } catch { /* closing playback also revokes it */ }
        return;
      }
      if (result.playbackId !== playback.id || typeof result.generation !== 'string' || !result.generation.trim() || !Number.isFinite(result.offsetSeconds) || result.offsetSeconds < 0 || !Number.isFinite(result.durationSeconds) || result.durationSeconds < 0 || typeof result.manifestUrl !== 'string') {
        throw new Error('Respuesta de conversión no válida');
      }
      const manifest = new URL(result.manifestUrl, window.location.origin);
      if (manifest.origin !== window.location.origin) throw new Error('El manifiesto debe proceder de la API local');
      this.transcodeOffset.set(result.offsetSeconds);
      this.transcodeDuration.set(result.durationSeconds);
      this.shownPosition.set(result.offsetSeconds);
      this.playbackState.resumePosition.set(0);
      this.activeManifest = manifest.href;
      this.transcoding.set(true);
      await this.loadMedia(manifest.href, 'hls');
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
    this.hls?.destroy(); this.hls = undefined;
    const video = this.videoRef?.nativeElement;
    if (video) { video.pause(); video.removeAttribute('src'); video.load(); }
  }

  private async initializePlayback(url: string, format: 'hls'|'file', contentKey: string): Promise<void> {
    if (this.playbackState.kind() !== 'live' && this.playbackState.resumePosition() <= 0) {
      try {
        const { progress } = await firstValueFrom(this.api.progress());
        const saved = progress.find(entry => entry.contentKey === contentKey);
        if (saved && saved.positionSeconds > 0) this.playbackState.resumePosition.set(saved.positionSeconds);
      } catch { /* playback remains available if library lookup is temporarily unavailable */ }
    }
    if (!this.closing) await this.loadMedia(url, format);
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
    this.mediaAttempt++;
    if (this.heartbeat) clearInterval(this.heartbeat);
    if (this.progressTimer) clearInterval(this.progressTimer);
    this.hls?.destroy(); this.hls = undefined;
    const video = this.videoRef?.nativeElement;
    if (video) { video.pause(); video.removeAttribute('src'); video.load(); }
  }
  ngOnDestroy(): void { if (!this.closing) { this.saveProgress(true); this.releaseMedia(); } }
}
