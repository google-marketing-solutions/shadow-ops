import {Routes} from '@angular/router';
import {ActionSetsPageComponent} from './pages/action-sets-page.component';
import {DashboardPageComponent} from './pages/dashboard-page.component';
import {ReviewPageComponent} from './pages/review-page.component';
import {WorkspacePageComponent} from './pages/workspace-page.component';

/**
 * Route declarations mapping URL paths to page components.
 */
export const routes: Routes = [
  {path: '', pathMatch: 'full', redirectTo: 'dashboard'},
  {path: 'dashboard', component: DashboardPageComponent},
  {path: 'workspaces/:workspaceId', component: WorkspacePageComponent},
  {path: 'review/:workspaceId', component: ReviewPageComponent},
  {path: 'action-sets', component: ActionSetsPageComponent},
  {path: 'action-sets/:actionSetId', component: ReviewPageComponent},
  {path: '**', redirectTo: 'dashboard'}
];
