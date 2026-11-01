import { Component, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { AuthService } from '../../core/auth.service';
import { ContentRoutes } from '../../core/content-routes';

@Component({
  standalone: true,
  imports: [FormsModule],
  template: `
    <main class="login-page">
      <div class="login-grain"></div>
      <header class="login-brand"><span class="brand-mark"><img src="/logo.png" alt="" width="36" height="19"></span><span>walac<span>tv</span></span></header>
      <section class="login-layout">
        <form class="login-card" (ngSubmit)="submit()">
          <h1>Inicia sesión</h1>
          <label for="username">Usuario</label>
          <input id="username" name="username" autocomplete="username" [(ngModel)]="username" required autofocus>
          <label for="password">Contraseña</label>
          <input id="password" name="password" type="password" autocomplete="current-password" [(ngModel)]="password" required>
          @if (error()) { <div class="form-error" role="alert">{{ error() }}</div> }
          <button class="button-primary login-submit" type="submit" [disabled]="busy()">
            {{ busy() ? 'Entrando…' : 'Entrar' }} <span aria-hidden="true"><svg><use href="/icons.svg#arrow-right"/></svg></span>
          </button>
        </form>
      </section>
    </main>
  `,
  styles: [`
    :host{display:block;min-height:100vh}.login-page{min-height:100vh;position:relative;overflow:hidden;background:#11120f;color:#f0eee8;padding:34px clamp(24px,6vw,96px);display:flex;flex-direction:column;isolation:isolate}.login-page:before{content:"";position:absolute;z-index:-1;inset:0;background:none}.login-grain{position:absolute;z-index:-1;inset:0;opacity:.13;pointer-events:none;background-image:url("data:image/svg+xml,%3Csvg viewBox='0 0 180 180' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='.9' numOctaves='3' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)' opacity='.18'/%3E%3C/svg%3E")}.login-brand{display:flex;align-items:center;gap:11px;font-weight:700;font-size:18px;letter-spacing:-.7px}.login-brand>span:last-child span{color:#c6f268}.brand-mark{width:33px;height:33px;display:grid;place-content:center;border-radius:11px;background:#c6f268;color:#171913;font-weight:800;font-size:21px;letter-spacing:-2px}.brand-mark span{color:#667b38}.login-layout{width:min(1140px,100%);margin:auto;display:grid;grid-template-columns:1.2fr .8fr;gap:10vw;align-items:center;padding:76px 0}.eyebrow,.card-kicker{font-size:10px;letter-spacing:1.7px;color:#c6f268;font-weight:700}.eyebrow{display:flex;align-items:center;gap:10px}.live-dot{width:7px;height:7px;background:#c6f268;border-radius:50%;box-shadow:0 0 13px #c6f268}.login-story h1{font-size:clamp(56px,7.1vw,94px);line-height:.99;letter-spacing:-5px;margin:28px 0 26px;font-weight:500}.login-story h1 em{font-family:inherit;font-style:normal;font-weight:650;color:#c6f268;letter-spacing:inherit}.story-copy{color:#a5a69c;max-width:370px;line-height:1.8;font-size:15px;text-wrap:pretty}.story-index{display:flex;align-items:center;gap:13px;margin-top:73px;font-size:9px;letter-spacing:1.5px;color:#77796f}.story-index span:first-child{color:#c6f268}.story-index i{height:1px;width:44px;background:#44473e}.login-card{padding:38px 38px 30px;background:rgba(29,31,26,.82);border:1px solid rgba(255,255,255,.09);border-radius:17px;box-shadow:0 28px 90px #0005;backdrop-filter:blur(16px)}.login-card h2{font-size:29px;font-weight:500;letter-spacing:-1px;margin:11px 0 9px}.card-hint{color:#92948a;font-size:12px;line-height:1.7;margin:0 0 29px}.login-card label{display:block;font-size:11px;color:#c4c5bb;margin:16px 0 8px}.login-card input{width:100%;height:46px;border:1px solid #3a3d34;background:#171914;border-radius:8px;color:#f5f4ee;padding:0 13px;outline:none}.login-card input:focus{border-color:#9dbd5a;box-shadow:0 0 0 3px #c6f2681a}.login-card input::placeholder{color:#64675e}.button-primary{border:0;background:#c6f268;color:#1a1d14;border-radius:8px;font-size:12px;font-weight:700;padding:14px 17px;cursor:pointer;transition:transform .18s,background .18s}.button-primary:hover{background:#d3ff79;transform:translateY(-1px)}.button-primary:disabled{opacity:.55;cursor:wait;transform:none}.login-submit{margin-top:24px;width:100%;display:flex;justify-content:space-between;align-items:center}.login-submit span{font-size:17px}.secure-note{margin-top:22px;text-align:center;color:#777a70;font-size:10px}.secure-note span{color:#c6f268;margin-right:5px}.form-error{margin-top:15px;padding:10px 12px;border-radius:7px;background:#522e28;color:#ffc0a9;font-size:12px}.login-footer{display:flex;justify-content:space-between;color:#66695f;font-size:9px;letter-spacing:1.25px}@media(max-width:760px){.login-page{padding:22px}.login-layout{grid-template-columns:1fr;gap:38px;padding:55px 0}.login-story h1{font-size:61px;letter-spacing:-3.5px;margin:20px 0}.login-story h1 em{letter-spacing:inherit}.story-index{display:none}.login-card{padding:28px 24px}.login-footer span:last-child{display:none}}
  `],
})
export class LoginComponent {
  private readonly auth = inject(AuthService);
  private readonly router = inject(Router);
  private readonly route = inject(ActivatedRoute);
  private readonly contentRoutes = inject(ContentRoutes);
  username = '';
  password = '';
  readonly busy = signal(false);
  readonly error = signal('');

  async submit(): Promise<void> {
    if (this.busy()) return;
    this.busy.set(true); this.error.set('');
    try {
      await this.auth.signIn(this.username.trim(), this.password);
      this.password = '';
      await this.router.navigateByUrl(this.contentRoutes.returnAfterLogin(this.route.snapshot.queryParamMap.get('returnUrl')));
    } catch (err: unknown) {
      this.error.set(this.message(err, 'No se pudo iniciar sesión. Revisa tus datos e inténtalo de nuevo.'));
      this.password = '';
    } finally { this.busy.set(false); }
  }

  private message(err: unknown, fallback: string): string {
    if (typeof err === 'object' && err !== null && 'error' in err) {
      const body = (err as { error?: { error?: { message?: string } } }).error;
      return body?.error?.message || fallback;
    }
    return fallback;
  }
}
