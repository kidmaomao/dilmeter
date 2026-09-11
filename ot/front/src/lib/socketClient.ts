import * as protocols from '@/protocols';
import { resolveWebSocketUrl } from './webSocketUrl';

export class SocketClient {
    private socket?: WebSocket;
    private reconnectTimer?: number;
    private suspended = false;

    // true -> connected, false -> disconnected or connecting
    public onConnect?: (isConnected: boolean) => void;
    public onEvent?: (events: protocols.eventBase[]) => void;

    private readonly url: string;

    constructor(url: string) {
        this.url = resolveWebSocketUrl(url);
    }

    public get isOpen(): boolean {
        return this.socket?.readyState === WebSocket.OPEN;
    }

    public get isSuspended(): boolean {
        return this.suspended;
    }

    /**
     * Re-establish the live event stream after a minimized/suspended renderer
     * resumes.  Calling this repeatedly is safe: an in-flight connection is
     * never replaced by a second WebSocket.
     */
    public ensureConnected(): void {
        if (this.suspended) return;
        const state = this.socket?.readyState;
        if (state === WebSocket.OPEN || state === WebSocket.CONNECTING) return;
        this.connect();
    }

    public connect() {
        if (this.suspended) return;
        const existingState = this.socket?.readyState;
        if (existingState === WebSocket.OPEN || existingState === WebSocket.CONNECTING) {
            return;
        }

        if (this.reconnectTimer !== undefined) {
            window.clearTimeout(this.reconnectTimer);
            this.reconnectTimer = undefined;
        }

        const s = this.socket = new WebSocket(this.url);

        setTimeout(() => {
            if (s.readyState == WebSocket.CONNECTING) {
                console.log('connection timeout...');
                s.close();
            }
        }, 5000);

        s.onopen = () => {
            console.log("socket open");
            this.onConnect?.(true);
        };

        s.onmessage = e => {
            const events = JSON.parse(e.data) as protocols.eventBase[];
            // console.log(events);
            this.onEvent?.(events);
        };

        s.onerror = e => {
            console.log('socket error', e);
            s.close();

            // setTimeout(openSocket, 0);
        };

        s.onclose = e => {
            console.log('socket close', e);
            if (this.socket !== s) return;
            this.onConnect?.(false);
            this.socket = undefined;
            if (this.suspended) return;
            this.reconnectTimer = window.setTimeout(() => {
                this.reconnectTimer = undefined;
                this.ensureConnected();
            }, 500);
        };
    }

    /**
     * Stop feeding a renderer that the user cannot see. Native reminder
     * processing continues independently; the DPS view is rebuilt on resume.
     */
    public suspend(): void {
        this.suspended = true;
        if (this.reconnectTimer !== undefined) {
            window.clearTimeout(this.reconnectTimer);
            this.reconnectTimer = undefined;
        }
        const current = this.socket;
        this.socket = undefined;
        if (current && current.readyState !== WebSocket.CLOSED) current.close();
        this.onConnect?.(false);
    }

    public resume(): void {
        if (!this.suspended) {
            this.ensureConnected();
            return;
        }
        this.suspended = false;
        this.ensureConnected();
    }
}
