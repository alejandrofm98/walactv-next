import { Injectable } from '@angular/core';
import { CatalogKind, CatalogItem, Episode, Source } from '../models';

export interface ParsedContentRef { slug: string; id: string; }
export interface ParsedEpisodeRef extends ParsedContentRef { season: number; episode: number; }

@Injectable({ providedIn: 'root' })
export class ContentRoutes {
  private static readonly slugPattern = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
  private static readonly originPattern = /^[A-Za-z0-9_-]{16}$/;
  private static readonly idPattern = /^[0-9]{1,32}$/;

  slug(title: string, kind: CatalogKind | 'episode'): string {
    const visible = kind === 'live' ? title.replace(/#{2,}/g, ' ') : title;
    const slug = visible.normalize('NFKD').replace(/\p{M}/gu, '').toLowerCase()
      .replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 80).replace(/-+$/g, '');
    return slug || (kind === 'live' ? 'canal' : kind === 'movie' ? 'pelicula' : kind === 'series' ? 'serie' : 'episodio');
  }

  parseRef(value: string | null): ParsedContentRef | null {
    value = this.decodeSegment(value);
    if (!value) return null;
    const split = value.lastIndexOf('-');
    if (split < 1) return null;
    const slug = value.slice(0, split), id = value.slice(split + 1);
    return this.validSlug(slug) && ContentRoutes.idPattern.test(id) ? { slug, id } : null;
  }

  parseEpisodeRef(value: string | null): ParsedEpisodeRef | null {
    value = this.decodeSegment(value);
    if (!value) return null;
    const match = /^(t([0-9]{1,6})e([0-9]{1,6}))-([0-9]{1,32})$/.exec(value);
    if (!match) return null;
    return { slug: match[1], season: Number(match[2]), episode: Number(match[3]), id: match[4] };
  }

  validSlug(value: string): boolean { return value.length >= 1 && value.length <= 80 && ContentRoutes.slugPattern.test(value); }
  validOrigin(value: string | null): value is string { return !!value && ContentRoutes.originPattern.test(value); }

  private decodeSegment(value: string | null): string | null {
    if (!value || /%2f|%5c/i.test(value)) return null;
    let decoded: string;
    try { decoded = decodeURIComponent(value); } catch { return null; }
    return /[%/\\\u0000-\u001f]/.test(decoded) ? null : decoded;
  }

  async origin(sourceId: string): Promise<string> {
    const digest = new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(sourceId)));
    let binary = '';
    for (const byte of digest.slice(0, 12)) binary += String.fromCharCode(byte);
    return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '');
  }

  async item(source: Source, item: CatalogItem): Promise<string> {
    const prefix = item.kind === 'live' ? '/directo' : item.kind === 'movie' ? '/peliculas' : '/series';
    return `${prefix}/${this.slug(item.title, item.kind)}-${item.id}?o=${await this.origin(source.id)}`;
  }

  async movie(source: Source, item: CatalogItem, watch = false): Promise<string> {
    const base = await this.item(source, item);
    return watch ? `${base.slice(0, base.indexOf('?'))}/ver${base.slice(base.indexOf('?'))}` : base;
  }

  async series(source: Source, item: CatalogItem, season?: number | null): Promise<string> {
    const base = await this.item(source, item);
    return season === undefined || season === null ? base : `${base}&temporada=${encodeURIComponent(String(season))}`;
  }

  async episode(source: Source, series: CatalogItem, episode: Episode, season?: number | null): Promise<string> {
    const slug = `t${episode.season}e${episode.episode}`;
    const origin = await this.origin(source.id);
    const suffix = season === undefined || season === null ? '' : `&temporada=${encodeURIComponent(String(season))}`;
    return `/series/${this.slug(series.title, 'series')}-${series.id}/episodios/${slug}-${episode.id}?o=${origin}${suffix}`;
  }

  movieRef(ref: ParsedContentRef, origin: string): string {
    return `/peliculas/${encodeURIComponent(ref.slug)}-${ref.id}?o=${origin}`;
  }

  seriesRef(ref: ParsedContentRef, origin: string, season?: number | null): string {
    return `/series/${encodeURIComponent(ref.slug)}-${ref.id}?o=${origin}${season === undefined || season === null ? '' : `&temporada=${encodeURIComponent(String(season))}`}`;
  }

  async canonical(kind: CatalogKind, source: Source, item: CatalogItem, options: { watch?: boolean; season?: number | null; episode?: Episode; series?: CatalogItem } = {}): Promise<string> {
    if (kind === 'movie') return this.movie(source, item, options.watch);
    if (kind === 'live') return this.item(source, item);
    if (options.episode && options.series) return this.episode(source, options.series, options.episode, options.season);
    return this.series(source, item, options.season);
  }

  returnAfterLogin(value: unknown): string {
    if (typeof value !== 'string' || !value.startsWith('/') || value.startsWith('//') || value.includes('\\') || value.includes('#') || /[\u0000-\u001f]/.test(value)) return '/';
    if ((value.match(/\?/g) || []).length > 1) return '/';
    const [path, query = ''] = value.split('?');
    if (!path || path.startsWith('//') || /%2f|%5c/i.test(path)) return '/';
    const rawParts = path.split('/').slice(1);
    if (rawParts.some(part => !part)) return '/';
    const parts = rawParts.map(part => this.decodeSegment(part));
    if (parts.some(part => !part)) return '/';
    const decodedParts = parts as string[];
    let ok = false;
    if (decodedParts[0] === 'directo' && decodedParts.length === 2) ok = !!this.parseRef(decodedParts[1]);
    if (decodedParts[0] === 'peliculas' && (decodedParts.length === 2 || decodedParts.length === 3 && decodedParts[2] === 'ver')) ok = !!this.parseRef(decodedParts[1]);
    if (decodedParts[0] === 'series' && (decodedParts.length === 2 || decodedParts.length === 4 && decodedParts[2] === 'episodios')) ok = !!this.parseRef(decodedParts[1]) && (decodedParts.length === 2 || !!this.parseEpisodeRef(decodedParts[3]));
    if (decodedParts[0] === 'sources' && decodedParts.length === 4 && ['live', 'movies', 'series'].includes(decodedParts[2])) {
      ok = /^[A-Za-z0-9._-]{1,128}$/.test(decodedParts[1]) && ContentRoutes.idPattern.test(decodedParts[3]);
    }
    if (decodedParts[0] === 'sources' && decodedParts.length === 5 && decodedParts[2] === 'movies' && decodedParts[4] === 'watch') {
      ok = /^[A-Za-z0-9._-]{1,128}$/.test(decodedParts[1]) && ContentRoutes.idPattern.test(decodedParts[3]);
    }
    if (decodedParts[0] === 'sources' && decodedParts.length === 6 && decodedParts[2] === 'series' && decodedParts[4] === 'episodes') {
      ok = /^[A-Za-z0-9._-]{1,128}$/.test(decodedParts[1]) && ContentRoutes.idPattern.test(decodedParts[3]) && ContentRoutes.idPattern.test(decodedParts[5]);
    }
    if (decodedParts.length === 1 && decodedParts[0] === 'player') ok = true;
    if (!ok || /^\/(?:login)(?:[/?#]|$)/i.test(path)) return '/';
    const params = new URLSearchParams(query);
    const keys: string[] = [];
    params.forEach((_value, key) => keys.push(key));
    if (new Set(keys).size !== keys.length || keys.some(key => !['o', 'temporada', 'season'].includes(key))) return '/';
    const legacy = decodedParts[0] === 'sources';
    if (legacy && keys.some(key => key !== 'season')) return '/';
    if (!legacy && keys.includes('season')) return '/';
    if (legacy && keys.includes('season')) {
      const isSeries = decodedParts[2] === 'series' && (decodedParts.length === 4 || decodedParts.length === 6 && decodedParts[4] === 'episodes');
      if (!isSeries || !/^[0-9]{1,6}$/.test(params.get('season') || '')) return '/';
    }
    if (decodedParts[0] === 'series' && (decodedParts.length === 2 || decodedParts.length === 4 && decodedParts[2] === 'episodios') && keys.includes('temporada')) {
      const season = params.get('temporada') || '';
      if (!/^[0-9]{1,6}$/.test(season)) return '/';
    } else if (keys.includes('temporada')) return '/';
    if (decodedParts[0] !== 'sources' && decodedParts[0] !== 'player') {
      if (!this.validOrigin(params.get('o'))) return '/';
    } else if (decodedParts[0] === 'player' && keys.length) return '/';
    return value;
  }
}
