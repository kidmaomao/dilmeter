type ExternalLinkHost = Window & {
    __dilmeterOpenExternal?: (url: string) => Promise<unknown>;
};

/** Desktop links use Windows' registered browser; ordinary web pages keep anchors. */
export async function openExternalLink(event: MouseEvent, onError: (message: string) => void): Promise<void> {
    const open = (window as ExternalLinkHost).__dilmeterOpenExternal;
    if (typeof open !== 'function') return;
    const url = (event.currentTarget as HTMLAnchorElement).href;
    // Suppress WebView's new-window action before awaiting the native bridge.
    event.preventDefault();
    onError('');
    try {
        await open(url);
    } catch {
        onError('无法打开系统默认浏览器，请复制链接到浏览器中打开。');
    }
}
