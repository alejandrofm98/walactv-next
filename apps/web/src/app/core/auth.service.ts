import { Injectable, inject, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { ApiService } from './api.service';
import { PlaybackStateService } from './playback-state.service';
import { CatalogViewStateService } from './catalog-view-state.service';
import { User } from '../models';

@Injectable({ providedIn: 'root' })
export class AuthService {
  private readonly api = inject(ApiService);
  private readonly playback = inject(PlaybackStateService);
  private readonly catalogView = inject(CatalogViewStateService);
  readonly user = signal<User | null>(null);
  private initialized?: Promise<boolean>;

  initialize(): Promise<boolean> {
    this.initialized ??= firstValueFrom(this.api.me()).then(session => {
      this.user.set(session.user);
      this.api.setCsrf(session.csrfToken);
      return true;
    }).catch(() => {
      this.user.set(null);
      this.api.clearCsrf();
      this.catalogView.clearSession();
      return false;
    });
    return this.initialized;
  }

  async signIn(username: string, password: string): Promise<void> {
    const session = await firstValueFrom(this.api.login(username, password));
    this.user.set(session.user);
    this.api.setCsrf(session.csrfToken);
    this.initialized = Promise.resolve(true);
  }

  async signOut(): Promise<void> {
    try { await firstValueFrom(this.api.logout()); }
    finally {
      this.user.set(null);
      this.api.clearCsrf();
      this.initialized = Promise.resolve(false);
      this.playback.clear();
      this.catalogView.clearSession();
    }
  }

  expire(): void {
    this.user.set(null);
    this.api.clearCsrf();
    this.initialized = Promise.resolve(false);
    this.playback.clear();
    this.catalogView.clearSession();
  }
}
