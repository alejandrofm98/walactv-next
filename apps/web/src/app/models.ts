export type CatalogKind = 'live' | 'movie' | 'series';
export type PlaybackKind = 'live' | 'movie' | 'episode';
export type LibraryKind = PlaybackKind;
export type LibrarySection = CatalogKind | 'favorites' | 'acestream';

export interface User { id: string; username: string; role: string; }
export interface Session { user: User; csrfToken: string; }
export interface Source { id: string; label: string; accountKey: string; expiresAt: string; saved?: boolean; }
export interface Category { id: string; name: string; }
export interface CatalogItem { id: string; title: string; kind: CatalogKind; containerExtension?: string; contentKey?: string; categoryId: string; language: string; imageUrl?: string; }
export interface CatalogPage { items: CatalogItem[]; total: number; page: number; pageSize: number; }
export interface CatalogLanguage { id: string; name: string; }
export interface CatalogStatus { status: 'empty' | 'syncing' | 'ready' | 'error'; updatedAt?: string; errorCode?: string; counts: Record<CatalogKind, number>; }
export interface Episode { id: string; title: string; season: number; episode: number; containerExtension: string; contentKey?: string; }
export interface Playback { id: string; url: string; format: 'hls' | 'file'; title: string; contentKey: string; expiresAt: string; }
export interface VodTrack { id: string; type: 'audio' | 'subtitle'; language?: string; label: string; codec?: string; selectable: boolean; unavailable?: string; }
export interface VodTracks { status: 'complete' | 'partial' | 'unavailable'; revision?: string; tracks: VodTrack[]; reason?: string; }
export interface TranscodePlayback { playbackId: string; generation: string; manifestUrl: string; offsetSeconds: number; durationSeconds: number; audioTrackId?: string; subtitleTrackId?: string; }
export interface PlaybackDiagnostic { cause: string; upstreamStatus?: number; }
export interface Favorite { id: string; contentKey: string; title: string; kind: LibraryKind; }
export interface Progress { contentKey: string; title: string; kind: LibraryKind; positionSeconds: number; durationSeconds: number; updatedAt: string; }
export interface AceChannel { id: string; name: string; contentId: string; }
export interface ApiErrorBody { error?: { code?: string; message?: string; }; }
export type MetadataStatus = 'loading' | 'ready' | 'stale' | 'unresolved' | 'ambiguous' | 'disabled' | 'unavailable';
export interface MetadataPerson { name: string; character?: string; job?: string; }
export interface MetadataGenre { id: number; name: string; }
export interface SpanishMetadata { id: number; kind: 'movie' | 'series'; title?: string; originalTitle?: string; overview?: string; releaseDate?: string; year?: number; genres?: MetadataGenre[]; runtime?: number; rating?: number; ratingSource?: string; posterUrl?: string; backdropUrl?: string; credits?: MetadataPerson[]; }
export interface MetadataEpisode { number: number; name?: string; overview?: string; stillUrl?: string; airDate?: string; }
export interface SpanishMetadataSeason { number: number; name?: string; overview?: string; episodes: MetadataEpisode[]; }
export interface MetadataResponse { status: MetadataStatus; language?: string; source?: string; metadata?: SpanishMetadata; season?: SpanishMetadataSeason; }
