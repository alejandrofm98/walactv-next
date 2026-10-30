import { Injectable, signal } from '@angular/core';
import { Playback } from '../models';

@Injectable({ providedIn: 'root' })
export class PlaybackStateService {
  readonly playback = signal<Playback | null>(null);
  readonly title = signal('');
  readonly kind = signal<'live' | 'movie' | 'episode'>('live');
  readonly sourceId = signal('');
  readonly seriesId = signal<string | undefined>(undefined);
  readonly itemId = signal('');
  readonly contentKey = signal('');
  readonly resumePosition = signal(0);
  set(playback: Playback, sourceId: string, kind: 'live' | 'movie' | 'episode', itemId: string, seriesId?: string, resumePosition = 0): void {
    this.playback.set(playback); this.title.set(playback.title); this.sourceId.set(sourceId);
    this.kind.set(kind); this.itemId.set(itemId); this.seriesId.set(seriesId); this.contentKey.set(playback.contentKey);
    this.resumePosition.set(resumePosition);
  }
  clear(): void { this.playback.set(null); this.resumePosition.set(0); }
}
