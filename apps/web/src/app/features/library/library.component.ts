import { Component, DestroyRef, ElementRef, Injector, OnInit, ViewChild, afterNextRender, computed, inject, signal } from '@angular/core';
import { HttpErrorResponse } from '@angular/common/http';
import { FormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import { RouterLink } from '@angular/router';
import { Subscription, forkJoin, switchMap, takeWhile, timer } from 'rxjs';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ApiService } from '../../core/api.service';
import { AuthService } from '../../core/auth.service';
import { CatalogViewStateService } from '../../core/catalog-view-state.service';
import { PlaybackStateService } from '../../core/playback-state.service';
import { AceChannel, CatalogItem, CatalogKind, CatalogLanguage, CatalogStatus, Category, Episode, Favorite, LibrarySection, MetadataResponse, PlaybackKind, Progress, Source } from '../../models';

const SECTIONS: { id: LibrarySection; label: string; glyph: string; group: string }[] = [
  { id: 'live', label: 'En directo', glyph: '◉', group: 'TU TELEVISIÓN' },
  { id: 'movie', label: 'Películas', glyph: '▰', group: 'TU BIBLIOTECA' },
  { id: 'series', label: 'Series', glyph: '▤', group: 'TU BIBLIOTECA' },
  { id: 'favorites', label: 'Mi lista', glyph: '♡', group: 'TU BIBLIOTECA' },
  { id: 'acestream', label: 'Ace Stream', glyph: '↗', group: 'REPRODUCTORES' },
];

@Component({
  standalone: true,
  imports: [FormsModule, RouterLink],
  template: `
    <div class="app-frame">
      <aside class="sidebar">
        <a class="brand" routerLink="/" (click)="selectSection('live')"><span class="brand-mark">w<span>·</span></span><span>walac<span>tv</span></span></a>
        <div class="sidebar-rule"></div>
        @for (group of groups(); track group.name) {
          <p class="nav-label">{{ group.name }}</p>
          @for (item of group.items; track item.id) {
            <button class="nav-link" [class.active]="section() === item.id" (click)="selectSection(item.id)"><span class="nav-glyph">{{ item.glyph }}</span>{{ item.label }} @if(item.id === 'live'){<i class="nav-live"></i>}</button>
          }
        }
        <div class="sidebar-bottom">
          <div class="source-status" [class.connected]="source()"><span class="status-ring"></span><div><strong>{{ source() ? (source()?.saved ? 'Fuente guardada' : 'Fuente conectada') : 'Sin fuente IPTV' }}</strong><small>{{ source()?.label || 'Añade tu lista privada' }}</small></div></div>
          @if (source()) { <button class="text-action" (click)="openDeleteConfirmation($event)">Eliminar fuente guardada</button> }
          <div class="profile-line"><span class="avatar">{{ initials() }}</span><span class="profile-name">{{ auth.user()?.username }}</span><button class="logout" title="Cerrar sesión" aria-label="Cerrar sesión" (click)="logout()">↗</button></div>
        </div>
      </aside>

      <main #mainContent class="main-area" tabindex="-1">
        <header class="topbar">
          <div class="mobile-brand"><span class="brand-mark">w<span>·</span></span>walac<span>tv</span></div>
          <div class="breadcrumb"><span>INICIO</span><b>/</b>{{ sectionLabel().toUpperCase() }}</div>
          <div class="topbar-right"><button class="user-pill" aria-label="Cerrar sesión" (click)="logout()"><span>{{ initials() }}</span>Cerrar sesión</button></div>
        </header>

        <nav class="mobile-nav" aria-label="Secciones">
          @for (item of sections; track item.id) { <button [class.active]="section() === item.id" (click)="selectSection(item.id)"><span>{{ item.glyph }}</span>{{ item.label }}</button> }
          @if(source()) { <button class="mobile-source" (click)="openDeleteConfirmation($event)"><span>◌</span>Eliminar fuente</button> }
        </nav>

        @if (sourcesFailed() && section() !== 'acestream' && section() !== 'favorites') {
          <section class="source-recovery source-recovery-error"><span class="recovery-error-icon">!</span><strong>No se pudo recuperar tu fuente guardada</strong><small>No se ha cambiado tu lista. Comprueba que el servicio está disponible e inténtalo de nuevo.</small><button class="lime-button" [disabled]="sourcesLoading()" (click)="loadSources()">{{ sourcesLoading() ? 'Reintentando…' : 'Reintentar' }}</button></section>
        } @else if (sourcesLoading() && section() !== 'acestream' && section() !== 'favorites') {
          <section class="source-recovery"><span class="loader"></span><strong>Cargando fuente…</strong></section>
        } @else if (!source() && section() !== 'acestream' && section() !== 'favorites') {
          <section class="welcome-layout">
            <form class="connect-card" (ngSubmit)="connect()">
              <h1>Añade tu lista</h1>
              <p>Admite listas M3U y enlaces Xtream, como get.php.</p>
              <label for="playlist">URL M3U o Xtream</label>
              <div class="url-field"><span aria-hidden="true">↗</span><input id="playlist" type="text" inputmode="url" autocomplete="off" spellcheck="false" [(ngModel)]="playlistUrl" name="playlist" required></div>
              <div class="privacy-note"><span aria-hidden="true">◈</span><span>Se guarda cifrada y no se vuelve a mostrar.</span></div>
              @if (error()) { <div class="inline-error" role="alert">{{ error() }}</div> }
              <button class="lime-button connect-button" [disabled]="busy() || !playlistUrl.trim()">{{ busy() ? 'Comprobando lista…' : 'Conectar lista' }} <span>→</span></button>
            </form>
          </section>
        } @else if (section() === 'acestream') {
          <section class="content-page">
            <div class="page-heading"><div><h1>Ace <em>Stream</em></h1></div><span class="page-count">CATÁLOGO DE PRUEBA</span></div>
            <div class="notice-card"><span class="notice-icon" aria-hidden="true">↗</span><div><strong>Necesitas la app Ace Stream</strong><p>Al abrir un canal se inicia la app en Windows, Linux o Android. Ace Stream no está disponible en iPhone ni iPad.</p></div></div>
            @if (error()) { <div class="inline-error" role="alert">{{ error() }}</div> }
            <div class="ace-grid">@for (channel of aceChannels(); track channel.id) { <article class="ace-card"><div class="ace-wave" aria-hidden="true">AS</div><div class="ace-info"><h3>{{ channel.name }}</h3><code>{{ channel.contentId.slice(0, 12) }}…</code></div><button (click)="openAce(channel)" aria-label="Abrir {{ channel.name }}">↗</button></article> }</div>
            @if (!aceChannels().length && !busy()) { <div class="empty-state"><strong>No hay canales Ace Stream.</strong></div> }
            <div class="manual-ace"><div><strong>ID de Ace Stream</strong><small>40 caracteres hexadecimales.</small></div><form (ngSubmit)="openAceId()"><input name="aceId" [(ngModel)]="aceId" maxlength="40" placeholder="ID de 40 caracteres" autocomplete="off" aria-label="ID de Ace Stream"><button class="lime-button">Abrir canal →</button></form></div>
          </section>
        } @else if (section() === 'favorites') {
          <section class="content-page">
            <div class="page-heading"><div><h1>Mi <em>lista</em></h1></div><span class="page-count">{{ favorites().length }} ELEMENTOS</span></div>
            @if (favorites().length) { <div class="media-grid">@for (favorite of favorites(); track favorite.id; let i = $index) { <article class="media-card"><div class="poster poster-{{ i % 5 }} saved-poster"><span class="poster-tag">{{ kindName(favorite.kind) }}</span><span class="poster-title">{{ displayTitle(favorite.title, favorite.kind) }}</span></div><div class="media-meta"><strong>{{ displayTitle(favorite.title, favorite.kind) }}</strong><button class="favorite active" aria-label="Quitar de mi lista" (click)="removeFavorite(favorite)">♥</button></div></article> }</div> }
            @else { <div class="empty-state"><strong>No hay elementos guardados</strong><button class="outline-button" (click)="selectSection('live')">Ver canales →</button></div> }
            @if (progress().length) { <section class="continue-row"><div class="section-heading"><div><h2>Seguir viendo</h2></div></div><div class="progress-list">@for (entry of progress().slice(0,8); track entry.contentKey) { @if (source()) { <button class="progress-item" (click)="resume(entry)"><span class="progress-play">▶</span><span><b>{{ entry.title }}</b><small>{{ formatTime(entry.positionSeconds) }} de {{ formatTime(entry.durationSeconds) }}</small></span><i><span [style.width.%]="entry.durationSeconds ? entry.positionSeconds / entry.durationSeconds * 100 : 0"></span></i></button> } @else { <div class="progress-item static-progress"><span class="progress-play">◷</span><span><b>{{ entry.title }}</b><small>{{ formatTime(entry.positionSeconds) }} de {{ formatTime(entry.durationSeconds) }} · Conecta tu fuente para reanudar</small></span><i><span [style.width.%]="entry.durationSeconds ? entry.positionSeconds / entry.durationSeconds * 100 : 0"></span></i></div> } }</div></section> }
          </section>
        } @else {
          <section class="content-page">
            <header class="catalog-heading" [class.catalog-heading-live]="section() === 'live'">
              <h1>{{ sectionLabel() }}</h1>
            </header>
            <div class="catalog-status" [class.status-syncing]="catalogStatus()?.status === 'syncing'" [class.status-error]="catalogStatus()?.status === 'error'" aria-live="polite">
              <span class="status-orb"></span><div class="status-copy"><strong>{{ catalogStatusLabel() }}</strong><small>{{ catalogStatusDetail() }}</small></div>
              <button class="refresh-catalog" [disabled]="catalogStatus()?.status === 'syncing'" (click)="refreshCatalog()">{{ catalogStatus()?.status === 'syncing' ? 'Actualizando…' : 'Actualizar lista' }} <span>↻</span></button>
            </div>
            <div class="catalog-toolbar"><div class="catalog-results"><span class="live-indicator" [class.is-live]="section() === 'live'"></span><span class="result-count">{{ total() }} {{ total() === 1 ? 'título' : 'títulos' }}</span></div><div class="catalog-actions"><label class="search-box"><span aria-hidden="true">⌕</span><input type="search" maxlength="150" placeholder="Buscar por nombre…" [ngModel]="search()" (ngModelChange)="onSearch($event)" aria-label="Buscar por nombre en todo el catálogo"></label><select aria-label="Idioma aproximado" [ngModel]="languageId()" (ngModelChange)="chooseLanguage($event)"><option value="">Todos los idiomas</option>@for (language of languages(); track language.id) {<option [value]="language.id">{{ language.name }}</option>}</select><select aria-label="Categoría" [ngModel]="categoryId()" (ngModelChange)="chooseCategory($event)"><option value="">Todas las categorías</option>@for (cat of categories(); track cat.id) {<option [value]="cat.id">{{ cat.name }}</option>}</select></div></div>
            <p class="language-note">Los idiomas se infieren de las etiquetas del proveedor; pueden no coincidir con las pistas de audio.</p>
            @if (error()) { <div class="inline-error" role="alert">{{ error() }} <button (click)="loadItems()">Reintentar</button></div> }
            @if (busy()) { <div class="loading-grid">@for (_ of [1,2,3,4,5,6,7,8]; track $index) {<div class="skeleton"></div>}</div> }
            @else if (items().length) {
              <div class="media-grid">@for (item of items(); track item.kind + ':' + item.id + ':' + item.categoryId; let i = $index) {<article class="media-card"><button class="poster poster-{{ i % 8 }}" [class.poster-live]="section() === 'live'" (click)="selectItem(item, $event)" [attr.aria-label]="(section() === 'live' ? 'Reproducir ' : item.kind === 'series' ? 'Ver episodios de ' : 'Reproducir ') + displayTitle(item.title, item.kind)"><span class="poster-tag">{{ section() === 'live' ? 'EN VIVO' : item.containerExtension?.toUpperCase() || (section() === 'series' ? 'SERIE' : 'PELÍCULA') }}</span>@if (item.imageUrl) { <img class="poster-art" [src]="item.imageUrl" [alt]="'Imagen de ' + displayTitle(item.title, item.kind)" loading="lazy" (error)="hideBrokenImage($event)"> }<span class="poster-title">{{ displayTitle(item.title, item.kind) }}</span><span class="play-disc">{{ section() === 'live' ? '▶' : '＋' }}</span></button><div class="media-meta"><strong>{{ displayTitle(item.title, item.kind) }}</strong><button class="favorite" [class.active]="isFavorite(item.contentKey)" [disabled]="!item.contentKey || item.kind === 'series'" [attr.aria-label]="isFavorite(item.contentKey) ? 'Quitar de mi lista' : 'Añadir a mi lista'" (click)="toggleFavorite(item)">{{ isFavorite(item.contentKey) ? '♥' : '♡' }}</button></div><small class="media-sub">{{ categoryName(item.categoryId) }} <span>·</span> {{ languageName(item.language) }}</small></article> }</div>
              @if (pageCount() > 1) { <div class="pagination"><button [disabled]="page() <= 1" (click)="setPage(page()-1)">← Anterior</button><span>{{ page() }} <i>/</i> {{ pageCount() }}</span><button [disabled]="page() >= pageCount()" (click)="setPage(page()+1)">Siguiente →</button></div> }
            } @else if (!error()) { <div class="empty-state compact"><span>⌕</span><strong>{{ search() ? 'Sin resultados' : 'Sin contenido' }}</strong><small>{{ search() ? 'Cambia la búsqueda.' : 'Prueba otra categoría o idioma.' }}</small></div> }
            @if (progress().length) {<section class="continue-row"><div class="section-heading"><div><h2>Seguir viendo</h2></div></div><div class="progress-list">@for (entry of progress().slice(0,4); track entry.contentKey) {<button class="progress-item" (click)="resume(entry)"><span class="progress-play">▶</span><span><b>{{ entry.title }}</b><small>{{ formatTime(entry.positionSeconds) }} de {{ formatTime(entry.durationSeconds) }}</small></span><i><span [style.width.%]="entry.durationSeconds ? entry.positionSeconds / entry.durationSeconds * 100 : 0"></span></i></button>}</div></section> }
          </section>
        }
      </main>
      @if (detailFor()) { <div class="modal-backdrop" (click)="closeDetail()" (keydown)="handleDetailKeydown($event, 'detail')"><section #detailDialog class="detail-dialog" role="dialog" aria-modal="true" aria-labelledby="detail-title" (click)="$event.stopPropagation()" tabindex="-1"><button #detailClose class="detail-close" aria-label="Cerrar ficha" (click)="closeDetail()">×</button><div class="detail-layout"><div class="detail-art">@if (metadata()?.metadata?.posterUrl || detailFor()?.imageUrl) {<img [src]="metadata()?.metadata?.posterUrl || detailFor()?.imageUrl" [alt]="'Imagen de ' + (metadata()?.metadata?.title || detailFor()?.title)" (error)="hideBrokenImage($event)">} @else {<span aria-hidden="true">{{ detailFor()?.kind === 'series' ? '▤' : '▰' }}</span>}</div><div class="detail-copy"><h2 id="detail-title">{{ metadata()?.metadata?.title || detailFor()?.title }}</h2><p class="detail-source-note">{{ metadataStatusText() }}</p>@if (metadata()?.metadata?.overview) {<p class="metadata-overview">{{ metadata()?.metadata?.overview }}</p>}@if (metadata()?.metadata?.genres?.length) {<p class="metadata-detail-line"><b>Géneros:</b> {{ metadataGenreText() }}</p>}@if (metadata()?.metadata?.credits?.length) {<p class="metadata-detail-line"><b>Reparto y equipo:</b> {{ metadataCreditText() }}</p>}<div class="detail-facts"><span><small>CATEGORÍA</small><b>{{ categoryName(detailFor()?.categoryId || '') }}</b></span><span><small>IDIOMA APROXIMADO</small><b>{{ languageName(detailFor()?.language || 'und') }}</b></span>@if (metadata()?.metadata?.year) {<span><small>AÑO</small><b>{{ metadata()?.metadata?.year }}</b></span>}@if (metadata()?.metadata?.runtime) {<span><small>DURACIÓN</small><b>{{ metadata()?.metadata?.runtime }} min</b></span>}@if (metadata()?.metadata?.rating !== undefined) {<span><small>VALORACIÓN TMDB</small><b>{{ metadata()?.metadata?.rating }}/10</b></span>}@if (detailFor()?.containerExtension) {<span><small>FORMATO</small><b>{{ detailFor()?.containerExtension?.toUpperCase() }}</b></span>}</div>
              @if (detailFor()?.kind === 'movie') {<button class="detail-play" (click)="playDetail()" [disabled]="busy()">{{ busy() ? 'Preparando…' : '▶  Reproducir película' }}</button>}
              @else {<div class="season-heading"><label for="season-select">Temporada</label>@if (seasons().length) {<select id="season-select" [ngModel]="selectedSeason()" (ngModelChange)="selectSeason(+$event)">@for (season of seasons(); track season) {<option [ngValue]="season">Temporada {{ season }}</option>}</select>}</div>
                @if (metadataSeason()?.season?.overview) {<p class="metadata-overview">{{ metadataSeason()?.season?.overview }}</p>}@else if (metadataSeasonStatusText()) {<p class="modal-status">{{ metadataSeasonStatusText() }}</p>}
                @if (episodesLoading()) {<p class="modal-status">Cargando episodios…</p>}@else if (filteredEpisodes().length) {<div class="episode-list">@for (ep of filteredEpisodes(); track ep.id) {<div class="episode-row"><button class="episode-main" (click)="playEpisode(ep)"><span class="episode-number">T{{ ep.season }} · E{{ ep.episode }}</span><span><b>{{ metadataEpisodeTitle(ep) }}</b><small>{{ ep.containerExtension.toUpperCase() }}</small></span><i>▶</i></button><button class="favorite" [class.active]="isFavorite(ep.contentKey)" [disabled]="!ep.contentKey" [attr.aria-label]="isFavorite(ep.contentKey) ? 'Quitar de mi lista' : 'Añadir a mi lista'" (click)="toggleEpisodeFavorite(ep)">{{ isFavorite(ep.contentKey) ? '♥' : '♡' }}</button></div>}</div>}@else{<p class="modal-status">{{ episodes().length ? 'No hay episodios en esta temporada.' : 'No hay episodios disponibles.' }}</p>}
              }
              @if (metadata()?.metadata) {<div class="metadata-credit"><a href="https://www.themoviedb.org" target="_blank" rel="noopener noreferrer" aria-label="TMDB, sitio oficial"><img class="tmdb-logo" alt="TMDB" src="https://www.themoviedb.org/assets/2/v4/logos/v2/blue_square_2-d537fb228cf3ded904ef09b136fe3fec72548ebc1fea3fbbd1ad9e36364db38b.svg"></a><small>Datos de TMDB · TMDB no respalda esta aplicación.</small></div>}
            </div></div></section></div> }
      @if (confirmDeleteSource()) { <div class="modal-backdrop" (click)="closeDeleteConfirmation()" (keydown)="handleDetailKeydown($event, 'delete')"><section #deleteDialog class="episode-modal confirm-source-modal" role="alertdialog" aria-modal="true" aria-labelledby="delete-source-title" aria-describedby="delete-source-description" (click)="$event.stopPropagation()"><div class="modal-heading"><div><p class="eyebrow"><span class="eyebrow-line"></span> ELIMINAR FUENTE</p><h2 id="delete-source-title">¿Eliminar esta lista guardada?</h2></div><button class="cancel-delete-close" aria-label="Cancelar" (click)="closeDeleteConfirmation()">×</button></div><p id="delete-source-description">Se borrará la URL cifrada y el catálogo guardado de tu cuenta. Para volver a usar esta lista tendrás que introducir su URL otra vez.</p>@if(error()){<div class="inline-error" role="alert">{{ error() }}</div>}<div class="confirm-actions"><button #cancelDelete class="outline-button" (click)="closeDeleteConfirmation()">Cancelar</button><button class="delete-button" [disabled]="busy()" (click)="disconnect()">{{ busy() ? 'Eliminando…' : 'Eliminar fuente' }}</button></div></section></div> }
      @if (notice()) { <div class="toast" role="status">{{ notice() }}<button (click)="notice.set('')">×</button></div> }
    </div>
  `,
  styles: [`
    :host{display:block;min-height:100vh}.app-frame{min-height:100vh;background:#11120f;color:#f0eee8;display:flex}.sidebar{width:238px;flex:0 0 238px;padding:26px 18px 17px;border-right:1px solid #272922;display:flex;flex-direction:column;background:#141510}.brand{display:flex;align-items:center;gap:10px;text-decoration:none;color:#f2f0e8;font-size:17px;font-weight:700;letter-spacing:-.7px;padding:0 8px}.brand>span:last-child span,.mobile-brand>span:last-child{color:#c6f268}.brand-mark{width:31px;height:31px;display:grid;place-content:center;border-radius:10px;background:#c6f268;color:#171913;font-weight:800;font-size:20px;letter-spacing:-2px}.brand-mark span{color:#657a37}.sidebar-rule{height:1px;background:#292b25;margin:30px 7px 26px}.nav-label{font-size:10px;color:#6d7066;letter-spacing:1.5px;margin:0 10px 12px}.nav-link{width:100%;display:flex;align-items:center;gap:12px;padding:11px 12px;border:0;border-radius:7px;text-align:left;color:#9b9d92;background:none;font:inherit;font-size:13px;min-height:42px;cursor:pointer;margin:2px 0;transition:all .16s}.nav-link:hover{color:#eeeee8;background:#1c1e18}.nav-link.active{background:#292c21;color:#d7f596}.nav-glyph{width:18px;text-align:center;font-size:16px;color:#9da18f}.nav-link.active .nav-glyph{color:#c6f268}.nav-live{width:5px;height:5px;background:#b9f169;border-radius:50%;margin-left:auto;box-shadow:0 0 8px #c6f268}.sidebar-bottom{margin-top:auto}.source-status{display:flex;align-items:center;gap:10px;padding:13px 10px;background:#1c1e19;border:1px solid #292b24;border-radius:8px;margin-bottom:10px}.status-ring{width:8px;height:8px;flex:0 0 8px;border-radius:50%;background:#77796f}.source-status.connected .status-ring{background:#c6f268;box-shadow:0 0 9px #c6f268}.source-status strong,.source-status small{display:block}.source-status strong{font-size:12px;font-weight:600}.source-status small{font-size:11px;color:#81847a;margin-top:4px;max-width:160px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.text-action{border:0;background:none;color:#9b9d92;font:inherit;font-size:12px;min-height:40px;padding:5px 10px 13px;cursor:pointer}.text-action:hover{color:#c6f268}.profile-line{border-top:1px solid #292b25;margin-top:10px;padding:17px 5px 0;display:flex;align-items:center;gap:9px}.avatar,.user-pill>span{width:28px;height:28px;border-radius:50%;background:#35392c;color:#d3f283;display:grid;place-items:center;font-size:10px;font-weight:700}.profile-name{font-size:11px;color:#c3c4bb;overflow:hidden;text-overflow:ellipsis}.logout{margin-left:auto;border:0;background:none;color:#8b8e82;font-size:16px;cursor:pointer;transform:rotate(45deg)}.main-area{min-width:0;flex:1}.topbar{height:62px;border-bottom:1px solid #272922;display:flex;align-items:center;justify-content:space-between;padding:0 clamp(22px,4vw,54px)}.breadcrumb{font-size:10px;letter-spacing:1.3px;color:#a3a59a}.breadcrumb span{color:#66695f}.breadcrumb b{margin:0 9px;color:#4c4e47}.topbar-right{display:flex;align-items:center;gap:25px}.secure{font-size:9px;letter-spacing:1.1px;color:#7d8075}.secure i{display:inline-block;width:5px;height:5px;background:#c6f268;border-radius:50%;margin-right:6px;vertical-align:1px}.user-pill{display:flex;align-items:center;gap:8px;border:0;background:none;color:#c6c7bd;font:inherit;font-size:10px;cursor:pointer}.user-pill>span{width:23px;height:23px}.user-pill b{color:#888a80;font-weight:400}.mobile-brand{display:none}.mobile-nav{display:none}.source-recovery{min-height:calc(100vh - 130px);display:flex;flex-direction:column;align-items:center;justify-content:center;color:#d7d8ce}.source-recovery .loader{margin-bottom:18px}.source-recovery strong{font-size:14px;font-weight:550}.source-recovery small{margin-top:7px;color:#909389;font-size:11px}.source-recovery-error .recovery-error-icon{width:36px;height:36px;margin-bottom:14px;display:grid;place-items:center;border:1px solid #9c6559;border-radius:50%;color:#e8a28e;font-size:17px}.source-recovery-error small{max-width:430px;text-align:center;line-height:1.6}.source-recovery-error .lime-button{margin-top:19px;min-width:125px;min-height:42px}.content-page{max-width:1420px;margin:0 auto;padding:36px clamp(22px,4vw,54px) 70px}.welcome-layout{max-width:1240px;margin:0 auto;padding:clamp(50px,8vh,96px) clamp(22px,6vw,80px) 45px;display:grid;grid-template-columns:1.12fr .88fr;gap:8vw;align-items:center;min-height:calc(100vh - 160px)}.eyebrow{font-size:10px;letter-spacing:1.6px;color:#b6b8aa;display:flex;align-items:center;gap:10px}.eyebrow-line{width:22px;height:1px;background:#c6f268;display:inline-block}.welcome-copy h1,.hero-copy h1,.page-heading h1{font-size:clamp(59px,7.5vw,94px);line-height:.98;letter-spacing:-5.5px;font-weight:450;margin:27px 0 23px}.welcome-copy h1 em,.hero-copy h1 em,.page-heading h1 em,.section-heading h2 em{font-family:Georgia,serif;color:#c6f268;font-weight:400}.welcome-description{max-width:365px;color:#97998e;font-size:13px;line-height:1.8}.feature-stamps{display:flex;gap:26px;margin-top:48px}.feature-stamps span{font-size:9px;color:#c6f268;letter-spacing:1px}.feature-stamps i{font-style:normal;color:#777a6f;margin-left:5px;font-size:8px}.connect-card{position:relative;max-width:410px;width:100%;padding:25px 26px 18px;border:1px solid #34372e;border-radius:12px;background:linear-gradient(147deg,#20221c,#181914 70%);box-shadow:0 32px 80px #0004;justify-self:end}.card-topline{display:flex;justify-content:space-between;color:#85887d;font-size:8px;letter-spacing:1.3px}.connect-icon{width:38px;height:38px;margin-top:31px;border:1px solid #56613d;border-radius:11px;display:grid;place-items:center;color:#c6f268;font-size:22px}.connect-card h2{font-size:23px;letter-spacing:-.8px;font-weight:500;margin:17px 0 8px}.connect-card>p{font-size:11px;color:#97998e;line-height:1.7;margin:0 0 24px}.connect-card label{font-size:12px;color:#c1c3b8;display:block;margin-bottom:8px}.url-field{height:44px;border:1px solid #393c33;border-radius:7px;background:#12130f;display:flex;align-items:center;padding:0 12px;gap:10px}.url-field span{color:#a6c967}.url-field input{border:0;background:transparent;outline:none;color:#eeefe8;flex:1;min-width:0;font-size:12px}.url-field input::placeholder{color:#666960}.privacy-note{display:flex;gap:8px;margin:11px 1px 18px;color:#9a9d91;font-size:10px;line-height:1.5}.privacy-note>span:first-child{color:#b6d872}.lime-button{background:#c6f268;border:0;color:#171913;border-radius:6px;padding:11px 13px;font:inherit;font-size:10px;font-weight:700;cursor:pointer;transition:background .15s,transform .15s}.lime-button:hover{background:#d9ff91;transform:translateY(-1px)}.lime-button:disabled{opacity:.5;cursor:wait;transform:none}.connect-button{width:100%;display:flex;justify-content:space-between;align-items:center}.connect-foot{border-top:1px solid #30322b;margin-top:20px;padding-top:13px;display:flex;justify-content:space-between;color:#66695f;font-size:8px;letter-spacing:1px}.welcome-bottom{max-width:1080px;margin:0 auto;padding:19px clamp(22px,6vw,80px);border-top:1px solid #292b25;display:grid;grid-template-columns:repeat(3,1fr);gap:25px}.welcome-bottom>div{display:grid;grid-template-columns:29px 1fr;column-gap:8px}.bottom-index{grid-row:span 2;color:#c6f268;font-size:9px;padding-top:2px}.welcome-bottom b{font-size:12px;font-weight:600}.welcome-bottom small{color:#77796f;font-size:9px;margin-top:6px}.inline-error{margin:13px 0;padding:11px 13px;color:#ffc0a9;background:#4a2a24;border-radius:7px;font-size:11px}.inline-error button{background:none;border:0;color:#ffe0d3;text-decoration:underline;cursor:pointer;margin-left:7px}.page-heading{display:flex;justify-content:space-between;align-items:end;margin:5px 0 27px}.page-heading .eyebrow{margin-bottom:0}.page-heading h1{font-size:53px;letter-spacing:-3px;margin:15px 0 9px}.subheading{color:#92948a;font-size:11px;margin:0}.page-count{color:#92958a;font-size:10px;letter-spacing:1.3px;margin-bottom:5px}.catalog-hero{position:relative;height:218px;border:1px solid #33362b;border-radius:12px;overflow:hidden;display:flex;align-items:center;padding:28px 38px;background:radial-gradient(ellipse at 83% 60%,#48583275,transparent 38%),linear-gradient(112deg,#25271f,#171914 62%,#20261a)}.catalog-hero.live-hero{background:radial-gradient(ellipse at 84% 39%,#8a734c55,transparent 39%),linear-gradient(112deg,#30251d,#1b1a16 60%,#262319)}.hero-copy{position:relative;z-index:1}.hero-copy .eyebrow{font-size:8px}.hero-copy h1{font-size:44px;letter-spacing:-2.8px;margin:13px 0 7px}.hero-copy>p:last-child{font-size:10px;color:#a0a295}.hero-side{margin-left:auto;position:relative;z-index:1;display:flex;flex-direction:column;align-items:center;gap:12px}.hero-ring{width:110px;height:110px;border:1px solid #c6f26861;border-radius:50%;display:grid;place-items:center;position:relative;color:#c6f268;font-family:Georgia,serif;font-size:26px}.hero-ring:before,.hero-ring:after{content:"";position:absolute;border-radius:50%;border:1px solid #c6f26825}.hero-ring:before{inset:9px}.hero-ring:after{inset:19px}.hero-side>span{font-size:8px;letter-spacing:.8px;color:#a5a795;max-width:145px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.hero-orbit{position:absolute;border:1px solid #d7f59610;border-radius:50%;width:460px;height:460px;right:-95px;top:-120px}.orbit-b{width:330px;height:330px;right:-30px;top:-55px}.catalog-toolbar{padding:24px 0 18px;display:flex;align-items:center;justify-content:space-between;gap:16px}.catalog-title{display:flex;align-items:center;gap:9px}.catalog-title strong{font-size:13px;font-weight:550}.result-count{font-size:10px;color:#797c70;border-left:1px solid #42443c;padding-left:10px}.live-indicator{width:6px;height:6px;background:#797c70;border-radius:50%}.live-indicator.is-live{background:#f17e6b;box-shadow:0 0 8px #f17e6b}.catalog-actions{display:flex;gap:9px}.search-box,.catalog-actions select{height:33px;border:1px solid #30332b;border-radius:5px;background:#191a16;color:#acafa2}.search-box{display:flex;align-items:center;padding:0 9px;gap:7px}.search-box span{font-size:19px;color:#9fa68c}.search-box input{width:145px;border:0;background:transparent;outline:0;color:#eee;font-size:12px}.search-box input::placeholder{color:#66695f}.catalog-actions select{padding:0 24px 0 9px;font:inherit;font-size:11px;min-height:40px;max-width:195px;outline:none}.media-grid{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:23px 15px}.media-card{min-width:0}.poster{position:relative;width:100%;aspect-ratio:2/2.83;overflow:hidden;border:1px solid #ffffff14;border-radius:6px;padding:15px;text-align:left;color:#fff;cursor:pointer;display:flex;flex-direction:column;justify-content:space-between;transition:transform .22s,border-color .22s;background:radial-gradient(ellipse at 65% 30%,#747c4b77,transparent 33%),linear-gradient(155deg,#34392b,#171914 75%)}.poster:hover{transform:translateY(-4px);border-color:#c6f26880}.poster:before{content:"";position:absolute;inset:0;background:linear-gradient(155deg,transparent 20%,#080907a8 95%)}.poster:after{content:"";position:absolute;inset:8px;border:1px solid #ffffff0c;border-radius:2px}.poster-tag,.poster-title,.play-disc{position:relative;z-index:1}.poster-tag{font-size:7px;letter-spacing:1px;color:#d5f38e;align-self:flex-start;background:#11130c9c;border:1px solid #c6f26840;border-radius:20px;padding:5px 7px}.poster-title{font-family:Georgia,serif;font-size:clamp(15px,1.65vw,22px);line-height:1.06;letter-spacing:-.5px;max-width:100%;overflow:hidden;display:-webkit-box;-webkit-line-clamp:4;-webkit-box-orient:vertical;text-shadow:0 2px 12px #000}.play-disc{align-self:flex-end;width:29px;height:29px;border-radius:50%;display:grid;place-items:center;background:#c6f268;color:#171913;font-size:10px;padding-left:2px;opacity:0;transform:scale(.85);transition:all .18s}.poster:hover .play-disc{opacity:1;transform:scale(1)}.poster-1{background:radial-gradient(ellipse at 20% 48%,#87664890,transparent 42%),linear-gradient(145deg,#45362c,#171614 75%)}.poster-2{background:radial-gradient(ellipse at 68% 65%,#53638c8c,transparent 42%),linear-gradient(155deg,#253041,#141719 75%)}.poster-3{background:radial-gradient(ellipse at 28% 26%,#8b505286,transparent 41%),linear-gradient(155deg,#432b30,#171517 75%)}.poster-4{background:radial-gradient(ellipse at 70% 26%,#92765080,transparent 38%),linear-gradient(150deg,#493c2b,#171714 74%)}.poster-5{background:radial-gradient(ellipse at 35% 60%,#4d715e91,transparent 40%),linear-gradient(145deg,#243c31,#141716 73%)}.poster-6{background:radial-gradient(ellipse at 68% 40%,#735c8e88,transparent 40%),linear-gradient(155deg,#3a2c49,#151419 72%)}.poster-7{background:radial-gradient(ellipse at 35% 24%,#68758d91,transparent 40%),linear-gradient(155deg,#2a3745,#151718 75%)}.media-meta{display:flex;align-items:center;justify-content:space-between;gap:5px;margin-top:9px}.media-meta strong{min-width:0;font-size:12px;font-weight:550;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:#deded5}.favorite{flex:0 0 40px;width:40px;height:40px;border:0;border-radius:50%;background:none;color:#777a70;font-size:15px;cursor:pointer}.favorite:hover,.favorite.active{color:#d98b7d}.media-sub{display:block;margin-top:3px;color:#92958a;font-size:10px}.loading-grid{display:grid;grid-template-columns:repeat(5,1fr);gap:15px}.skeleton{aspect-ratio:2/2.83;border-radius:6px;background:linear-gradient(100deg,#1c1e18 20%,#292b23 40%,#1c1e18 60%);background-size:200% 100%;animation:shine 1.3s linear infinite}@keyframes shine{to{background-position:-200% 0}}.empty-state{min-height:260px;display:flex;flex-direction:column;align-items:center;justify-content:center;text-align:center;color:#898c80;border:1px dashed #383a32;border-radius:9px}.empty-state>span{font-size:30px;color:#c6f268;margin-bottom:13px}.empty-state strong{font-size:13px;font-weight:550;color:#d5d6cc}.empty-state small{font-size:10px;margin-top:7px;color:#85877d}.empty-state.compact{min-height:180px;border:0}.outline-button{margin-top:19px;background:none;border:1px solid #4b523b;color:#c6f268;padding:10px 14px;border-radius:5px;font-size:10px;cursor:pointer}.pagination{display:flex;align-items:center;justify-content:center;gap:22px;margin:30px 0;color:#a9aa9f;font-size:10px}.pagination button{border:1px solid #36382f;border-radius:5px;color:#bfc0b7;background:#1a1b16;padding:8px 11px;font-size:9px;cursor:pointer}.pagination button:disabled{opacity:.4;cursor:default}.pagination i{font-style:normal;color:#71746a}.continue-row{border-top:1px solid #292b25;margin-top:39px;padding-top:23px}.section-heading span{color:#c6f268;font-size:8px;letter-spacing:1.3px}.section-heading h2{font-weight:450;font-size:22px;letter-spacing:-.7px;margin:9px 0 16px}.progress-list{display:flex;gap:10px;flex-wrap:wrap}.progress-item{flex:1;min-width:210px;display:grid;grid-template-columns:30px 1fr;align-items:center;column-gap:9px;text-align:left;padding:11px;border:1px solid #30322a;border-radius:7px;background:#1a1b16;color:#ddd;cursor:pointer}.progress-item.static-progress{cursor:default}.progress-play{grid-row:span 2;width:28px;height:28px;display:grid;place-items:center;background:#30372a;color:#c6f268;border-radius:50%;font-size:9px}.progress-item b,.progress-item small{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.progress-item b{font-size:10px}.progress-item small{margin-top:4px;font-size:8px;color:#84877c}.progress-item>i{grid-column:2;height:2px;margin-top:7px;background:#414238}.progress-item>i span{height:100%;display:block;background:#c6f268}.notice-card{display:flex;gap:14px;border:1px solid #3c4230;border-radius:8px;background:#1e2119;padding:15px 17px;margin:20px 0 22px}.notice-icon{width:29px;height:29px;border-radius:7px;background:#313b26;color:#c6f268;display:grid;place-items:center;flex:0 0 29px}.notice-card strong{font-size:11px;font-weight:600}.notice-card p{margin:6px 0 0;color:#92958a;font-size:10px;line-height:1.6;max-width:720px}.ace-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}.ace-card{display:flex;align-items:center;gap:13px;padding:14px;border:1px solid #30332a;border-radius:8px;background:#191a16}.ace-wave{width:43px;height:43px;border:1px solid #59633f;border-radius:50%;color:#c6f268;display:grid;place-items:center;font:italic 14px Georgia;flex:0 0 43px}.ace-info{min-width:0;flex:1}.ace-info>span{font-size:9px;letter-spacing:1.1px;color:#85887e}.ace-info h3{font-size:14px;font-weight:550;margin:5px 0}.ace-info code{font-size:10px;color:#797d70}.ace-card>button{width:40px;height:40px;border:1px solid #4a503b;border-radius:50%;background:none;color:#c6f268;cursor:pointer}.manual-ace{margin-top:25px;padding:18px;border:1px solid #30332a;border-radius:8px;display:flex;align-items:center;justify-content:space-between;gap:25px}.manual-ace>div strong,.manual-ace>div small{display:block}.manual-ace>div strong{margin:12px 0 5px;font-size:12px}.manual-ace>div small{font-size:9px;color:#85877d}.manual-ace form{display:flex;gap:8px}.manual-ace input{width:220px;background:#171813;border:1px solid #383b31;border-radius:5px;color:#efefe8;padding:0 10px;font-size:10px}.confirm-source-modal>p{color:#a4a69a;font-size:12px;line-height:1.7;padding:15px 2px 3px}.confirm-actions{display:flex;justify-content:flex-end;gap:10px;margin-top:17px}.confirm-actions .outline-button{margin:0;min-height:42px}.delete-button{min-height:42px;padding:0 16px;border:0;border-radius:5px;background:#a84d3e;color:#fff2ec;font-size:11px;font-weight:650;cursor:pointer}.delete-button:hover{background:#c25a48}.delete-button:disabled{opacity:.55;cursor:wait}.modal-backdrop{position:fixed;z-index:20;inset:0;background:#080907c9;display:grid;place-items:center;padding:20px}.episode-modal{width:min(540px,100%);max-height:min(80vh,720px);overflow:auto;background:#1a1b17;border:1px solid #44473c;border-radius:11px;padding:23px;box-shadow:0 25px 80px #0009}.modal-heading{display:flex;justify-content:space-between;align-items:start;border-bottom:1px solid #30322a;padding-bottom:16px}.modal-heading .eyebrow{margin:0}.modal-heading h2{font:24px Georgia,serif;margin:10px 0 0}.modal-heading>button{width:40px;height:40px;border:0;border-radius:50%;background:#292b24;color:#d5d6cc;font-size:19px;cursor:pointer}.episode-list{padding-top:7px}.episode-row{display:grid;grid-template-columns:1fr 40px;align-items:center;border-bottom:1px solid #292b25}.episode-main{width:100%;min-height:54px;display:grid;grid-template-columns:78px 1fr 25px;gap:12px;align-items:center;text-align:left;border:0;background:none;color:#e3e3da;padding:8px 3px;cursor:pointer}.episode-main:hover{background:#20221b}.episode-number{color:#c6f268;font-size:10px}.episode-row b,.episode-row small{display:block}.episode-row b{font-size:12px;font-weight:550}.episode-row small{font-size:10px;color:#85887c;margin-top:4px}.episode-row i{color:#c6f268;font-size:10px;font-style:normal}.modal-status{color:#aaa;font-size:11px;padding:22px 0}.metadata-overview{color:#c1c3b8;font-size:12px;line-height:1.7;margin:16px 0;white-space:pre-line}.metadata-detail-line{color:#b6b8ad;font-size:10px;line-height:1.6;margin:10px 0}.metadata-detail-line b{color:#777b6e;font-weight:600}.metadata-credit{display:flex;align-items:center;gap:10px;border-top:1px solid #30322a;padding-top:14px;margin-top:16px}.tmdb-logo{width:29px;height:22px;object-fit:contain}.metadata-credit small{font-size:9px;line-height:1.5;color:#92958a}.toast{position:fixed;z-index:25;right:25px;bottom:25px;max-width:340px;background:#293020;border:1px solid #647440;color:#e4efcc;border-radius:7px;padding:13px 15px;font-size:11px;box-shadow:0 8px 30px #0008}.toast button{margin-left:15px;background:none;border:0;color:#d0e4a9;cursor:pointer}
    @media(max-width:1080px){.sidebar{width:205px;flex-basis:205px}.media-grid,.loading-grid{grid-template-columns:repeat(4,minmax(0,1fr))}.welcome-layout{gap:5vw}.welcome-copy h1{font-size:72px}}
    @media(max-width:760px){.app-frame{display:block}.sidebar{display:none}.mobile-nav{display:flex;gap:5px;overflow-x:auto;padding:7px 12px;border-bottom:1px solid #292b25;background:#151610;scrollbar-width:none}.mobile-nav::-webkit-scrollbar{display:none}.mobile-nav button{flex:0 0 auto;min-height:42px;display:flex;align-items:center;gap:7px;padding:0 12px;border:1px solid transparent;border-radius:6px;background:none;color:#a5a79c;font-size:11px;white-space:nowrap}.mobile-nav button span{font-size:15px;color:#c6f268}.mobile-nav button.active{background:#292c21;border-color:#404535;color:#e1e9d0}.mobile-nav .mobile-source{color:#d7aa92}.mobile-nav .mobile-source span{color:#d7aa92}.topbar{height:56px;padding:0 18px}.mobile-brand{display:flex;align-items:center;gap:8px;font-size:14px;font-weight:700}.mobile-brand .brand-mark{width:26px;height:26px;font-size:17px}.breadcrumb{display:none}.topbar-right{gap:9px}.secure{display:none}.user-pill{font-size:0;min-width:44px;min-height:44px;justify-content:center}.user-pill b{font-size:13px}.user-pill>span{font-size:9px}.welcome-layout{padding:48px 22px 28px;display:block;min-height:auto}.welcome-copy h1{font-size:65px;letter-spacing:-4px;margin:21px 0 16px}.welcome-description{font-size:13px}.feature-stamps{margin-top:25px;margin-bottom:31px}.connect-card{max-width:none;padding:22px}.welcome-bottom{grid-template-columns:1fr;padding:16px 22px;gap:13px}.welcome-bottom>div{grid-template-columns:27px 1fr}.welcome-bottom small{margin-top:3px;font-size:10px}.content-page{padding:27px 17px 55px}.page-heading h1{font-size:45px}.catalog-hero{height:178px;padding:20px}.hero-copy h1{font-size:35px;letter-spacing:-2px}.hero-side{transform:scale(.75);transform-origin:right center;margin-right:-7px}.hero-side>span{max-width:100px;text-align:center;font-size:7px}.catalog-toolbar{align-items:flex-start;flex-direction:column;padding-top:19px}.catalog-actions{width:100%}.search-box{flex:1;height:40px}.search-box input{width:100%;font-size:12px}.catalog-actions select{max-width:48%;height:40px;font-size:11px}.media-grid,.loading-grid{grid-template-columns:repeat(3,minmax(0,1fr));gap:17px 10px}.poster{padding:10px}.poster-tag{font-size:8px;padding:4px 5px}.poster-title{font-size:15px}.media-meta strong{font-size:12px}.favorite{width:40px;height:40px;flex-basis:40px;font-size:18px}.media-sub{font-size:10px}.ace-grid{grid-template-columns:1fr}.ace-card>button{width:40px;height:40px;flex:0 0 40px}.manual-ace{align-items:stretch;flex-direction:column;gap:16px}.manual-ace form{display:grid;grid-template-columns:1fr auto}.manual-ace input{width:100%;min-width:0;min-height:42px;font-size:12px}.notice-card p{font-size:11px}.page-count{font-size:10px}.episode-modal{padding:17px}.episode-row{min-height:54px}.pagination{gap:12px}}
    @media(max-width:390px){.media-grid,.loading-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.catalog-actions select{font-size:8px}.welcome-copy h1{font-size:58px}.page-count{display:none}}

  `],
})
export class LibraryComponent implements OnInit {
  @ViewChild('mainContent') private mainContent?: ElementRef<HTMLElement>;
  @ViewChild('detailDialog') private detailDialog?: ElementRef<HTMLElement>;
  @ViewChild('detailClose') private detailClose?: ElementRef<HTMLButtonElement>;
  @ViewChild('deleteDialog') private deleteDialog?: ElementRef<HTMLElement>;
  @ViewChild('cancelDelete') private cancelDelete?: ElementRef<HTMLButtonElement>;
  readonly auth = inject(AuthService);
  private readonly api = inject(ApiService);
  private readonly router = inject(Router);
  private readonly injector = inject(Injector);
  private readonly playbackState = inject(PlaybackStateService);
  private readonly viewState = inject(CatalogViewStateService);
  private readonly destroyRef = inject(DestroyRef);
  readonly sections = SECTIONS;
  readonly groups = computed(() => [...new Set(SECTIONS.map(s => s.group))].map(name => ({ name, items: SECTIONS.filter(s => s.group === name) })));
  readonly section = this.viewState.section;
  readonly sectionLabel = computed(() => SECTIONS.find(s => s.id === this.section())?.label || 'Inicio');
  readonly source = signal<Source | null>(null);
  readonly sources = signal<Source[]>([]);
  readonly sourcesLoading = signal(false);
  readonly sourcesFailed = signal(false);
  playlistUrl = '';
  readonly busy = signal(false);
  readonly error = signal('');
  readonly notice = signal('');
  readonly confirmDeleteSource = signal(false);
  readonly categories = this.viewState.categories;
  readonly categoryId = this.viewState.categoryId;
  readonly languages = signal<CatalogLanguage[]>([]);
  readonly languageId = this.viewState.languageId;
  readonly catalogStatus = signal<CatalogStatus | null>(null);
  readonly catalogStatusLabel = computed(() => {
    const status = this.catalogStatus();
    if (!status || status.status === 'empty') return 'Prepara tu catálogo';
    if (status.status === 'syncing') return 'Actualizando catálogo';
    if (status.status === 'error') return 'No se pudo actualizar';
    return 'Catálogo guardado';
  });
  readonly catalogStatusDetail = computed(() => {
    const status = this.catalogStatus();
    if (!status || status.status === 'empty') return 'Se guardará en tu cuenta para que esté disponible al volver.';
    if (status.status === 'syncing') return `${this.statusCount(status)} elementos guardados · puedes seguir explorando`;
    if (status.status === 'error') return status.updatedAt ? `Se conserva la última copia · ${this.formatDate(status.updatedAt)}` : 'La lista sigue guardada; inténtalo de nuevo cuando quieras.';
    return `Actualizado ${status.updatedAt ? this.formatDate(status.updatedAt) : 'recientemente'} · ${this.statusCount(status)} elementos`;
  });
  readonly items = this.viewState.items;
  readonly total = this.viewState.total;
  readonly page = this.viewState.page;
  readonly pageSize = 40;
  readonly pageCount = computed(() => Math.max(1, Math.ceil(this.total() / this.pageSize)));
  readonly search = this.viewState.search;
  readonly favorites = signal<Favorite[]>([]);
  readonly progress = signal<Progress[]>([]);
  readonly detailFor = signal<CatalogItem | null>(null);
  readonly metadata = signal<MetadataResponse | null>(null);
  readonly metadataSeason = signal<MetadataResponse | null>(null);
  readonly episodesFor = signal<CatalogItem | null>(null);
  readonly episodes = signal<Episode[]>([]);
  readonly episodesLoading = signal(false);
  readonly selectedSeason = signal<number | null>(null);
  readonly seasons = computed(() => [...new Set(this.episodes().map(episode => episode.season))].sort((a, b) => a - b));
  readonly filteredEpisodes = computed(() => this.episodes().filter(episode => episode.season === this.selectedSeason()));
  readonly aceChannels = signal<AceChannel[]>([]);
  aceId = '';
  private searchTimer?: ReturnType<typeof setTimeout>;
  private categoryRequest?: Subscription;
  private itemsRequest?: Subscription;
  private episodesRequest?: Subscription;
  private metadataRequest?: Subscription;
  private metadataSeasonRequest?: Subscription;
  private catalogWatch?: Subscription;
  private pendingResume?: Progress;
  private detailTrigger: HTMLElement | null = null;
  private deleteTrigger: HTMLElement | null = null;

  constructor() {
    this.destroyRef.onDestroy(() => { if (this.searchTimer) clearTimeout(this.searchTimer); });
  }

  private focusAfterRender(focus: () => void): void {
    afterNextRender(focus, { injector: this.injector });
  }

  private restoreFocus(trigger: HTMLElement | null): void {
    this.focusAfterRender(() => {
      if (trigger?.isConnected && !(trigger instanceof HTMLButtonElement && trigger.disabled)) trigger.focus();
      else this.mainContent?.nativeElement.focus();
    });
  }

  openDeleteConfirmation(event: MouseEvent): void {
    this.deleteTrigger = event.currentTarget as HTMLElement;
    this.confirmDeleteSource.set(true);
    this.focusAfterRender(() => this.cancelDelete?.nativeElement.focus());
  }

  closeDeleteConfirmation(): void {
    if (this.busy() || !this.confirmDeleteSource()) return;
    this.confirmDeleteSource.set(false);
    const trigger = this.deleteTrigger;
    this.deleteTrigger = null;
    this.restoreFocus(trigger);
  }

  ngOnInit(): void {
    this.loadLibrary();
    this.loadSources();
  }

  loadSources(): void {
    if (this.sourcesLoading()) return;
    this.sourcesLoading.set(true); this.sourcesFailed.set(false); this.error.set('');
    this.api.sources().pipe(takeUntilDestroyed(this.destroyRef)).subscribe({ next: ({ sources }) => {
      this.sourcesLoading.set(false); this.sourcesFailed.set(false);
      const nextSource = sources[0] || null;
      if (this.viewState.setSourceId(nextSource?.id ?? null)) {
        this.languages.set([]); this.catalogStatus.set(null);
      }
      this.sources.set(sources); this.source.set(nextSource);
      if (this.source() && ['live', 'movie', 'series'].includes(this.section())) this.loadCatalog();
      else if (this.section() === 'acestream') this.loadAceChannels();
    }, error: () => {
      this.sourcesLoading.set(false); this.sourcesFailed.set(true); this.source.set(null); this.sources.set([]);
    } });
  }

  initials(): string { return (this.auth.user()?.username || 'W').slice(0, 2).toUpperCase(); }
  displayTitle(title: string, kind: CatalogKind | PlaybackKind): string {
    if (kind !== 'live') return title;
    return title.replace(/#{2,}/g, ' ').replace(/\s+/g, ' ').trim() || 'Canal sin nombre';
  }

  selectSection(section: LibrarySection): void {
    if (section !== this.section()) { this.search.set(''); this.categoryId.set(''); this.languageId.set(''); this.page.set(1); }
    this.section.set(section); this.error.set('');
    if (section === 'acestream') { this.itemsRequest?.unsubscribe(); this.categoryRequest?.unsubscribe(); this.loadAceChannels(); }
    else if (section === 'favorites') this.loadFavorites();
    else if (this.source()) this.loadCatalog();
  }

  connect(): void {
    const submitted = this.playlistUrl.trim();
    if (!submitted || this.busy()) return;
    this.busy.set(true); this.error.set('');
    this.api.connect(submitted).pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
      next: source => { this.playlistUrl = ''; this.viewState.setSourceId(source.id); this.languages.set([]); this.catalogStatus.set(null); this.source.set(source); this.sources.set([source]); this.notice.set('Lista conectada. Ya puedes explorar el catálogo.'); this.loadCatalog(); },
      error: err => { this.playlistUrl = ''; this.error.set(this.safeMessage(err, 'No se pudo conectar la lista. Comprueba la URL e inténtalo de nuevo.')); this.busy.set(false); },
      complete: () => this.busy.set(false),
    });
  }

  disconnect(): void {
    const source = this.source(); if (!source || this.busy()) return;
    this.busy.set(true); this.api.removeSource(source.id).pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
      next: () => { this.confirmDeleteSource.set(false); this.deleteTrigger = null; this.focusAfterRender(() => this.mainContent?.nativeElement.focus()); this.error.set(''); this.source.set(null); this.sources.set([]); this.viewState.setSourceId(null); this.languages.set([]); this.catalogStatus.set(null); this.notice.set('Fuente guardada eliminada.'); },
      error: err => { this.error.set(this.safeMessage(err, 'No se pudo desconectar la fuente.')); this.busy.set(false); },
      complete: () => this.busy.set(false),
    });
  }

  loadCatalog(): void {
    const source = this.source(), section = this.section();
    if (!source || !['live','movie','series'].includes(section)) return;
    this.error.set('');
    this.categoryRequest?.unsubscribe(); this.itemsRequest?.unsubscribe(); this.catalogWatch?.unsubscribe();
    this.catalogWatch = this.api.catalogStatus(source.id).pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
      next: status => {
        this.catalogStatus.set(status);
        if (status.status === 'empty') this.beginCatalogRefresh(source.id);
        else if (status.status === 'syncing') { this.loadCatalogFacets(source.id, section as CatalogKind); this.watchCatalog(source.id); }
        else if (status.status === 'ready' || status.status === 'error') {
          if (status.status === 'error') this.error.set('No se pudo completar la sincronización. Se conserva la última copia disponible.');
          this.loadCatalogFacets(source.id, section as CatalogKind);
        }
      }, error: err => this.error.set(this.safeMessage(err, 'No se pudo consultar el estado del catálogo.')),
    });
  }

  refreshCatalog(): void {
    const source = this.source(); if (!source) return;
    this.error.set('');
    this.catalogWatch?.unsubscribe();
    this.beginCatalogRefresh(source.id);
  }

  private beginCatalogRefresh(sourceId: string): void {
    this.catalogStatus.update(status => status ? { ...status, status: 'syncing' } : null);
    this.api.refreshCatalog(sourceId).pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
      next: status => { this.catalogStatus.set(status); this.watchCatalog(sourceId); },
      error: err => { this.catalogStatus.update(status => status ? { ...status, status: 'error' } : null); this.error.set(this.safeMessage(err, 'No se pudo iniciar la sincronización.')); },
    });
  }

  private watchCatalog(sourceId: string): void {
    this.catalogWatch?.unsubscribe();
    this.catalogWatch = timer(1200, 1800).pipe(
      switchMap(() => this.api.catalogStatus(sourceId)),
      takeWhile(status => status.status === 'syncing', true),
      takeUntilDestroyed(this.destroyRef),
    ).subscribe({
      next: status => {
        this.catalogStatus.set(status);
        if (status.status === 'ready') {
          const section = this.section();
          if (['live', 'movie', 'series'].includes(section)) this.loadCatalogFacets(sourceId, section as CatalogKind);
        } else if (status.status === 'error') this.error.set('No se pudo completar la sincronización. La copia guardada, si existe, sigue disponible.');
      }, error: err => this.error.set(this.safeMessage(err, 'Se perdió la conexión durante la sincronización.')),
    });
  }

  private loadCatalogFacets(sourceId: string, type: CatalogKind): void {
    this.busy.set(true);
    this.categoryRequest?.unsubscribe();
    this.categoryRequest = forkJoin({
      categories: this.api.categories(sourceId, type),
      languages: this.api.languages(sourceId, type),
    }).pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
      next: ({ categories, languages }) => {
        this.categories.set(categories.categories);
        this.languages.set(languages.languages);
        if (!categories.categories.some(category => category.id === this.categoryId())) this.categoryId.set('');
        if (!languages.languages.some(language => language.id === this.languageId())) this.languageId.set('');
        this.loadItems();
      }, error: err => { this.error.set(this.safeMessage(err, 'No se pudieron cargar los filtros.')); this.busy.set(false); },
    });
  }

  loadItems(): void {
    const source = this.source(), section = this.section();
    if (!source || !['live','movie','series'].includes(section)) return;
    this.busy.set(true); this.error.set('');
    this.itemsRequest?.unsubscribe();
    this.itemsRequest = this.api.items(source.id, section as CatalogKind, this.categoryId(), this.languageId(), this.page(), this.search()).pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
      next: result => { this.items.set(result.items); this.total.set(result.total); this.page.set(result.page); this.busy.set(false); },
      error: err => { this.items.set([]); this.error.set(this.safeMessage(err, 'No se pudo cargar el catálogo.')); this.busy.set(false); },
    });
  }

  chooseCategory(id: string): void { this.categoryId.set(id); this.page.set(1); this.loadItems(); }
  chooseLanguage(id: string): void { this.languageId.set(id); this.page.set(1); this.loadItems(); }
  setPage(page: number): void { this.page.set(page); this.loadItems(); }
  onSearch(value: string): void {
    this.search.set(value);
    this.itemsRequest?.unsubscribe();
    if (this.searchTimer) clearTimeout(this.searchTimer);
    this.searchTimer = setTimeout(() => { this.page.set(1); this.loadItems(); }, 280);
  }

  selectItem(item: CatalogItem, event?: MouseEvent): void {
    if (item.kind === 'live') { this.startPlayback(item.id, this.displayTitle(item.title, item.kind), 'live', 'live'); return; }
    this.detailTrigger = event?.currentTarget instanceof HTMLElement ? event.currentTarget : null;
    this.metadataRequest?.unsubscribe(); this.metadataSeasonRequest?.unsubscribe();
    this.metadata.set({ status: 'loading' }); this.metadataSeason.set(null);
    this.detailFor.set(item);
    this.focusAfterRender(() => this.detailClose?.nativeElement.focus());
    this.error.set('');
    const source = this.source();
    if (source) this.metadataRequest = this.api.metadata(source.id, item.kind as 'movie'|'series', item.id).pipe(takeUntilDestroyed(this.destroyRef)).subscribe({ next: value => this.metadata.set(value), error: () => this.metadata.set({ status: 'unavailable' }) });
    if (item.kind === 'series') {
      if (!source) return;
      this.episodesFor.set(item); this.episodes.set([]); this.selectedSeason.set(null); this.episodesLoading.set(true);
      this.episodesRequest?.unsubscribe();
      this.episodesRequest = this.api.episodes(source.id, item.id).pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
        next: ({ episodes }) => { this.episodes.set(episodes); const season = [...new Set(episodes.map(episode => episode.season))].sort((a, b) => a - b)[0] ?? null; this.selectedSeason.set(season); this.episodesLoading.set(false); if (season !== null) this.loadSeasonMetadata(item, season); },
        error: err => { this.error.set(this.safeMessage(err, 'No se pudieron cargar los episodios.')); this.episodesLoading.set(false); this.closeDetail(); },
      });
    } else this.episodesFor.set(null);
  }

  selectSeason(season: number): void { this.selectedSeason.set(season); const item = this.detailFor(); if (item?.kind === 'series') this.loadSeasonMetadata(item, season); }
  private loadSeasonMetadata(item: CatalogItem, season: number): void {
    const source = this.source(); if (!source) return;
    this.metadataSeasonRequest?.unsubscribe(); this.metadataSeason.set({ status: 'loading' });
    this.metadataSeasonRequest = this.api.seasonMetadata(source.id, item.id, season).pipe(takeUntilDestroyed(this.destroyRef)).subscribe({ next: value => { if (this.detailFor()?.id === item.id && this.selectedSeason() === season) this.metadataSeason.set(value); }, error: () => { if (this.selectedSeason() === season) this.metadataSeason.set({ status: 'unavailable' }); } });
  }
  metadataStatusText(): string {
    const value = this.metadata();
    if (!value || value.status === 'loading') return 'Abriendo ficha · la reproducción sigue disponible.';
    if (value.status === 'ready' || value.status === 'stale') return `Metadatos TMDB · ${value.language || 'es-ES'} · texto e imágenes consultados externamente.${value.status === 'stale' ? ' Mostrando copia guardada.' : ''}`;
    if (value.status === 'unresolved') return 'No se encontró una coincidencia verificable; se muestran los datos IPTV.';
    if (value.status === 'ambiguous') return 'Hay varias coincidencias posibles; se muestran los datos IPTV.';
    if (value.status === 'disabled') return 'Metadatos en español desactivados; se muestran los datos IPTV.';
    return 'Metadatos temporalmente no disponibles; la reproducción IPTV sigue disponible.';
  }
  metadataEpisodeTitle(episode: Episode): string { return this.metadataSeason()?.season?.episodes?.find(item => item.number === episode.episode)?.name || episode.title; }
  metadataGenreText(): string { return this.metadata()?.metadata?.genres?.map(genre => genre.name).join(' · ') || ''; }
  metadataCreditText(): string { return this.metadata()?.metadata?.credits?.map(person => person.character ? `${person.name} (${person.character})` : person.name).join(' · ') || ''; }
  metadataSeasonStatusText(): string { const value = this.metadataSeason(); if (!value || value.status === 'loading') return 'Consultando detalles de esta temporada…'; if (value.status === 'ready' || value.status === 'stale') return ''; return 'Detalles de temporada no disponibles; episodios IPTV conservados.'; }
  handleDetailKeydown(event: KeyboardEvent, kind: 'detail' | 'delete'): void {
    const dialog = kind === 'detail' ? this.detailDialog?.nativeElement : this.deleteDialog?.nativeElement;
    if (!dialog) return;
    if (event.key === 'Escape') {
      event.preventDefault(); event.stopPropagation();
      if (this.busy()) return;
      if (kind === 'detail') this.closeDetail();
      else this.closeDeleteConfirmation();
      return;
    }
    if (event.key !== 'Tab') return;
    event.stopPropagation();
    const controls = Array.from(dialog.querySelectorAll<HTMLElement>('button:not([disabled]), select:not([disabled]), input:not([disabled]), textarea:not([disabled]), a[href], [tabindex]:not([tabindex="-1"])'))
      .filter(control => control.getClientRects().length > 0);
    if (!controls.length) { event.preventDefault(); dialog.focus(); return; }
    const first = controls[0], last = controls[controls.length - 1];
    if (event.shiftKey && (document.activeElement === first || !dialog.contains(document.activeElement))) { event.preventDefault(); last.focus(); }
    else if (!event.shiftKey && (document.activeElement === last || !dialog.contains(document.activeElement))) { event.preventDefault(); first.focus(); }
  }
  closeDetail(restoreFocus = true): void {
    if (restoreFocus && this.busy()) return;
    this.episodesRequest?.unsubscribe(); this.metadataRequest?.unsubscribe(); this.metadataSeasonRequest?.unsubscribe(); this.episodesLoading.set(false); this.detailFor.set(null); this.episodesFor.set(null); this.episodes.set([]); this.selectedSeason.set(null); this.metadata.set(null); this.metadataSeason.set(null);
    const trigger = this.detailTrigger; this.detailTrigger = null;
    if (restoreFocus) this.restoreFocus(trigger);
  }
  playDetail(): void {
    const item = this.detailFor(); if (!item || item.kind !== 'movie') return;
    const trigger = this.detailTrigger;
    this.closeDetail(false);
    this.startPlayback(item.id, item.title, 'movie', 'movie', undefined, trigger);
  }

  playEpisode(episode: Episode): void {
    const parent = this.episodesFor(); if (!parent) return;
    const trigger = this.detailTrigger;
    this.closeDetail(false);
    this.startPlayback(episode.id, `${parent.title} · T${episode.season} E${episode.episode} · ${episode.title}`, 'episode', 'episode', parent.id, trigger);
  }

  private startPlayback(itemId: string, title: string, kind: PlaybackKind, mediaKind: 'live'|'movie'|'episode', seriesId?: string, focusOnError: HTMLElement | null = null): void {
    const source = this.source(); if (!source) return;
    this.busy.set(true); this.error.set('');
    this.api.openPlayback(source.id, kind, itemId, seriesId).pipe(takeUntilDestroyed(this.destroyRef)).subscribe({
      next: playback => {
        this.busy.set(false);
        const saved = this.progress().find(entry => entry.contentKey === playback.contentKey);
        const resumeAt = this.pendingResume?.contentKey === playback.contentKey ? this.pendingResume.positionSeconds : (saved?.positionSeconds || 0);
        this.pendingResume = undefined;
        const playbackTitle = playback.title || title;
        this.playbackState.set({ ...playback, title: mediaKind === 'live' ? this.displayTitle(playbackTitle, 'live') : playbackTitle }, source.id, mediaKind, itemId, seriesId, resumeAt);
        void this.router.navigateByUrl('/player');
      }, error: err => { this.busy.set(false); this.error.set(this.playbackStartMessage(err)); if (focusOnError) this.restoreFocus(focusOnError); },
    });
  }

  loadLibrary(): void {
    this.loadFavorites();
    this.api.progress().pipe(takeUntilDestroyed(this.destroyRef)).subscribe({ next: ({ progress }) => this.progress.set(progress), error: () => this.progress.set([]) });
  }
  loadFavorites(): void { this.api.favorites().pipe(takeUntilDestroyed(this.destroyRef)).subscribe({ next: ({ favorites }) => this.favorites.set(favorites), error: () => this.favorites.set([]) }); }
  isFavorite(contentKey?: string): boolean { return !!contentKey && this.favorites().some(f => f.contentKey === contentKey); }
  toggleFavorite(item: CatalogItem): void {
    if (item.kind === 'series') { this.notice.set('Abre un episodio para añadirlo a Mi lista.'); return; }
    const contentKey = item.contentKey;
    if (!contentKey) { this.notice.set('Este contenido todavía no tiene una clave de biblioteca disponible. Actualiza el catálogo.'); return; }
    const current = this.favorites().find(f => f.contentKey === contentKey);
    if (current) { this.removeFavorite(current); return; }
    this.api.addFavorite(contentKey, item.title, item.kind).pipe(takeUntilDestroyed(this.destroyRef)).subscribe({ next: () => { this.notice.set('Añadido a Mi lista.'); this.loadFavorites(); }, error: err => this.error.set(this.safeMessage(err, 'No se pudo guardar el favorito.')) });
  }
  toggleEpisodeFavorite(episode: Episode): void {
    if (!episode.contentKey) return;
    const current = this.favorites().find(f => f.contentKey === episode.contentKey);
    if (current) { this.removeFavorite(current); return; }
    this.api.addFavorite(episode.contentKey, episode.title, 'episode').pipe(takeUntilDestroyed(this.destroyRef)).subscribe({ next: () => { this.notice.set('Episodio añadido a Mi lista.'); this.loadFavorites(); }, error: err => this.error.set(this.safeMessage(err, 'No se pudo guardar el favorito.')) });
  }
  removeFavorite(favorite: Favorite): void { this.api.removeFavorite(favorite.id).pipe(takeUntilDestroyed(this.destroyRef)).subscribe({ next: () => { this.favorites.update(list => list.filter(f => f.id !== favorite.id)); this.notice.set('Quitado de Mi lista.'); }, error: err => this.error.set(this.safeMessage(err, 'No se pudo actualizar Mi lista.')) }); }
  resume(entry: Progress): void {
    this.pendingResume = entry;
    const section: LibrarySection = entry.kind === 'episode' ? 'series' : entry.kind;
    this.section.set(section); this.search.set(entry.title); this.categoryId.set(''); this.languageId.set(''); this.page.set(1);
    this.notice.set(`Busca “${entry.title}” en el catálogo. Al abrirlo retomará desde ${this.formatTime(entry.positionSeconds)}.`);
    this.loadCatalog();
  }
  formatTime(seconds: number): string { const s = Math.max(0, Math.floor(seconds)); return `${Math.floor(s / 60)}:${String(s % 60).padStart(2,'0')}`; }
  formatDate(value: string): string { return new Intl.DateTimeFormat('es-ES', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)); }
  statusCount(status: CatalogStatus): number { return status.counts.live + status.counts.movie + status.counts.series; }
  categoryName(id: string): string { return this.categories().find(category => category.id === id)?.name || 'Sin categoría'; }
  languageName(id: string): string { return this.languages().find(language => language.id === id)?.name || (id === 'und' ? 'Idioma desconocido' : id.toUpperCase()); }
  hideBrokenImage(event: Event): void { (event.currentTarget as HTMLImageElement).hidden = true; }
  kindName(kind: string): string { return kind === 'live' ? 'DIRECTO' : kind === 'episode' ? 'SERIE' : 'PELÍCULA'; }
  private loadAceChannels(): void { this.busy.set(true); this.api.aceChannels().pipe(takeUntilDestroyed(this.destroyRef)).subscribe({ next: ({ channels }) => { this.aceChannels.set(channels); this.busy.set(false); }, error: err => { this.error.set(this.safeMessage(err, 'No se pudo cargar Ace Stream.')); this.busy.set(false); } }); }
  openAce(channel: AceChannel): void { this.openAceContent(channel.contentId); }
  openAceId(): void { this.openAceContent(this.aceId.trim()); }
  private openAceContent(raw: string): void {
    const contentId = raw.toLowerCase().startsWith('acestream://') ? raw.slice(12) : raw;
    if (!/^[a-f\d]{40}$/i.test(contentId)) { this.error.set('El ID debe tener exactamente 40 caracteres hexadecimales.'); return; }
    this.error.set('');
    const ua = navigator.userAgent;
    if (/iPhone|iPad|iPod/i.test(ua)) { this.notice.set('Ace Stream no está disponible en iPhone o iPad. La reproducción IPTV sigue funcionando en Safari.'); return; }
    const href = /Android/i.test(ua) ? `intent://${contentId}#Intent;scheme=acestream;package=org.acestream.media;end` : `acestream://${contentId}`;
    window.location.assign(href);
    this.notice.set('Si no se abre, instala Ace Stream y vuelve a tocar el canal.');
    this.aceId = '';
  }
  async logout(): Promise<void> { try { await this.auth.signOut(); } finally { void this.router.navigateByUrl('/login'); } }
  private safeMessage(err: unknown, fallback: string): string {
    if (typeof err === 'object' && err !== null && 'error' in err) {
      const body = (err as { error?: { error?: { message?: string } } }).error;
      return body?.error?.message || fallback;
    }
    return fallback;
  }
  private playbackStartMessage(err: unknown): string {
    if (err instanceof HttpErrorResponse && err.status === 409 && err.error?.error?.code === 'PLAYBACK_LIMIT') {
      return 'La cuenta IPTV ya está reproduciendo en otro dispositivo. Espera a que termine esa sesión y vuelve a intentarlo.';
    }
    return this.safeMessage(err, 'No se pudo iniciar la reproducción.');
  }
}
