import { buildEventSnapshot } from "./buildEventSnapshot";
import type { WorkerInMessage, WorkerOutMessage } from "./workerProtocol";

self.onmessage = (e: MessageEvent<WorkerInMessage>) => {
    if (e.data.type !== "process") return;
    try {
        const snapshot = buildEventSnapshot(e.data.ndjson, (message) => self.postMessage(message));
        const message: WorkerOutMessage = { type: "done", snapshot };
        self.postMessage(message);
    } catch (error) {
        const message: WorkerOutMessage = { type: "error", message: String(error) };
        self.postMessage(message);
    }
};
