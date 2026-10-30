import { HttpErrorResponse, HttpInterceptorFn } from '@angular/common/http';
import { inject } from '@angular/core';
import { Router } from '@angular/router';
import { catchError, throwError } from 'rxjs';
import { AuthService } from './auth.service';

export const sessionExpiryInterceptor: HttpInterceptorFn = (request, next) => {
  const auth = inject(AuthService);
  const router = inject(Router);

  return next(request).pipe(catchError((error: unknown) => {
    const pathname = request.url.split('?')[0];
    const publicAuthRequest = pathname.endsWith('/auth/login') || pathname.endsWith('/auth/me');
    if (error instanceof HttpErrorResponse && error.status === 401 && pathname.startsWith('/api/v1/') && !publicAuthRequest && auth.user()) {
      auth.expire();
      if (router.url !== '/login') void router.navigateByUrl('/login');
    }
    return throwError(() => error);
  }));
};
