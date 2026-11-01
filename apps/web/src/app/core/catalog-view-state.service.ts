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
  captureContext(userId: string, focusKey?: string): CatalogNavigationContext | null {
    const section = this.section();
    if (section !== 'live' && section !== 'movie' && section !== 'series' && section !== 'favorites') return null;
    return { userId, sourceId: this.sourceId(), section, categoryId: this.categoryId(), languageId: this.languageId(), page: this.page(), search: this.search(), focusKey };
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
  focusKey?: string;
}
