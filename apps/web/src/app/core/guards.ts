import { inject } from '@angular/core';
import { CanActivateFn, Router, RouterStateSnapshot } from '@angular/router';
import { AuthService } from './auth.service';

export const authenticatedGuard: CanActivateFn = async (_route, state: RouterStateSnapshot) => {
  const auth = inject(AuthService);
  const router = inject(Router);
  return await auth.initialize() || router.createUrlTree(['/login'], { queryParams: { returnUrl: state.url } });
};

export const guestGuard: CanActivateFn = async () => {
  const auth = inject(AuthService);
  const router = inject(Router);
  return await auth.initialize() ? router.createUrlTree(['/']) : true;
};
