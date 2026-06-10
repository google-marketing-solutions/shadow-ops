import {CommonModule} from '@angular/common';
import {ChangeDetectionStrategy, Component, DestroyRef, inject} from '@angular/core';
import {takeUntilDestroyed} from '@angular/core/rxjs-interop';
import {RouterLink, RouterLinkActive, RouterOutlet} from '@angular/router';
import {ApiService} from './api.service';
import {SessionStateService} from './session-state.service';

/**
 * Root application component for ShadowOps.
 */
@Component({
  selector: 'app-root',
  standalone: true,
  imports: [CommonModule, RouterOutlet, RouterLink, RouterLinkActive],
  templateUrl: './app.component.ng.html',
  styleUrl: './app.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class AppComponent {
  private readonly api = inject(ApiService);
  private readonly sessionState = inject(SessionStateService);
  private readonly destroyRef = inject(DestroyRef);

  constructor() {
    this.api.health()
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: (response) => {
          if (response.shadowopsEnabled !== undefined) {
            this.sessionState.setShadowopsEnabled(response.shadowopsEnabled);
          }
        },
        error: () => {
          this.sessionState.setShadowopsEnabled(true);
        }
      });
  }
}
