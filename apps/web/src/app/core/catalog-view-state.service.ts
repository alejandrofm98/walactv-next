import { Injectable, signal } from '@angular/core';
import { CatalogItem, Category, LibrarySection } from '../models';

@Injectable({ providedIn: 'root' })
export class CatalogViewStateService {
  readonly section = signal<LibrarySection>('live');
  readonly sourceId = signal<string | null>(null);
  readonly categories = signal<Category[]>([]);
  readonly categoryId = signal('');
  readonly languageId = signal('');
  readonly items = signal<CatalogItem[]>([]);
  readonly total = signal(0);
  readonly page = signal(1);
  readonly search = signal('');
  readonly discoveryMode = signal(false);
  readonly discoveryGenre = signal('');
  readonly discoverySearchDraft = signal('');
  readonly discoverySearch = signal('');
  readonly discoveryCursors = signal<string[]>([]);
  readonly discoveryNextCursor = signal('');
  readonly discoveryHasMore = signal(false);
  readonly discoveryPartial = signal(false);
  readonly discoveryStatus = signal<'ready'|'unavailable'|'restart_required'>('ready');
  captureContext(userId: string, focusKey?: string): CatalogNavigationContext | null {
    const section = this.section();
    if (section !== 'live' && section !== 'movie' && section !== 'series' && section !== 'favorites') return null;
    return { userId, sourceId: this.sourceId(), section, categoryId: this.categoryId(), languageId: this.languageId(), page: this.page(), search: this.search(), discoveryMode: this.discoveryMode(), discoveryGenre: this.discoveryGenre(), discoverySearch: this.discoverySearch(), discoverySearchDraft: this.discoverySearchDraft(), discoveryCursors: this.discoveryCursors(), discoveryNextCursor: this.discoveryNextCursor(), discoveryHasMore: this.discoveryHasMore(), discoveryPartial: this.discoveryPartial(), focusKey };
  }

  restoreContext(context: unknown, userId: string, sourceId: string): boolean {
    if (!context || typeof context !== 'object') return false;
    const value = context as Partial<CatalogNavigationContext>;
    if (value.userId !== userId || value.sourceId !== sourceId || !['live', 'movie', 'series', 'favorites'].includes(value.section || '') || typeof value.categoryId !== 'string' || typeof value.languageId !== 'string' || typeof value.search !== 'string' || !Number.isInteger(value.page) || (value.page || 0) < 1) return false;
    this.section.set(value.section!);
    this.categoryId.set(value.categoryId);
    this.languageId.set(value.languageId);
    this.search.set(value.search.slice(0, 150));
    this.page.set(value.page!);
    this.discoveryMode.set(value.discoveryMode === true && (value.section === 'movie' || value.section === 'series'));
    this.discoveryGenre.set(typeof value.discoveryGenre === 'string' ? value.discoveryGenre : '');
    this.discoverySearch.set(typeof value.discoverySearch === 'string' ? value.discoverySearch.slice(0, 150) : '');
    this.discoverySearchDraft.set(typeof value.discoverySearchDraft === 'string' ? value.discoverySearchDraft.slice(0, 150) : this.discoverySearch());
    this.discoveryCursors.set(Array.isArray(value.discoveryCursors) && value.discoveryCursors.every(v => typeof v === 'string') ? value.discoveryCursors.slice(0, 100) : []);
    this.discoveryNextCursor.set(typeof value.discoveryNextCursor === 'string' ? value.discoveryNextCursor : '');
    this.discoveryHasMore.set(value.discoveryHasMore === true);
    this.discoveryPartial.set(value.discoveryPartial === true);
    return true;
  }
  setSourceId(sourceId: string | null): boolean {
    if (this.sourceId() === sourceId) return false;
    this.sourceId.set(sourceId);
    this.clearCatalog();
    return true;
  }

  clearCatalog(): void {
    this.categories.set([]);
    this.items.set([]);
    this.total.set(0);
    this.categoryId.set('');
    this.languageId.set('');
    this.page.set(1);
    this.search.set('');
    this.discoveryMode.set(false);
    this.discoveryGenre.set('');
    this.discoverySearchDraft.set('');
    this.discoverySearch.set('');
    this.discoveryCursors.set([]);
    this.discoveryNextCursor.set('');
    this.discoveryHasMore.set(false);
    this.discoveryPartial.set(false);
    this.discoveryStatus.set('ready');
  }

  clearSession(): void {
    this.clearCatalog();
    this.sourceId.set(null);
  }
}

export interface CatalogNavigationContext {
  userId: string;
  sourceId: string | null;
  section: LibrarySection;
  categoryId: string;
  languageId: string;
  page: number;
  search: string;
  discoveryMode?: boolean;
  discoveryGenre?: string;
  discoverySearchDraft?: string;
  discoverySearch?: string;
  discoveryCursors?: string[];
  discoveryNextCursor?: string;
  discoveryHasMore?: boolean;
  discoveryPartial?: boolean;
  focusKey?: string;
}
