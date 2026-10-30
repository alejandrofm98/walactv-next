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
