import { Routes } from '@angular/router';
import { authenticatedGuard, guestGuard } from './core/guards';

export const routes: Routes = [
  { path: 'login', canActivate: [guestGuard], loadComponent: () => import('./features/login/login.component').then(m => m.LoginComponent) },
  { path: '', canActivate: [authenticatedGuard], loadComponent: () => import('./features/library/library.component').then(m => m.LibraryComponent) },
  { path: 'player', canActivate: [authenticatedGuard], canDeactivate: [component => (component as { closeForNavigation: () => Promise<boolean> }).closeForNavigation()], loadComponent: () => import('./features/player/player.component').then(m => m.PlayerComponent) },
  { path: '**', redirectTo: '' },
];
