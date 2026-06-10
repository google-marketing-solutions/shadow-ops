import {Injectable} from '@angular/core';
import {Observable} from 'rxjs';
import {WorkspaceWsMessage} from './types';

/**
 * Service providing WebSocket streaming connections for live workspace sessions.
 */
@Injectable({providedIn: 'root'})
export class WorkspaceSocketService {
  connect(workspaceId: string): {
    messages$: Observable<WorkspaceWsMessage>;
    send: (payload: Record<string, unknown>) => void;
    close: () => void;
  } {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.port === '4200' ? `${window.location.hostname}:8080` : window.location.host;
    const socket = new WebSocket(`${protocol}//${host}/api/v1/workspaces/${workspaceId}/ws`);

    const messages$ = new Observable<WorkspaceWsMessage>((subscriber) => {
      socket.onmessage = (event) => {
        try {
          subscriber.next(JSON.parse(event.data as string) as WorkspaceWsMessage);
        } catch (error) {
          subscriber.error(error);
        }
      };
      socket.onerror = () => subscriber.error(new Error('Workspace socket error'));
      socket.onclose = () => subscriber.complete();

      return () => {
        if (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING) {
          socket.close();
        }
      };
    });

    return {
      messages$,
      send: (payload) => {
        if (socket.readyState === WebSocket.OPEN) {
          socket.send(JSON.stringify(payload));
        }
      },
      close: () => {
        if (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING) {
          socket.close();
        }
      }
    };
  }
}
