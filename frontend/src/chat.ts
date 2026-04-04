export interface ChatDeps {
    container: HTMLElement;
    input: HTMLTextAreaElement;
    sendBtn: HTMLButtonElement;
    clearBtn: HTMLButtonElement;
    captureBtn: HTMLButtonElement;
    getFrame: () => string;
    sendMessage: (text: string, frame: string) => Promise<string>;
    clearChat: () => Promise<void>;
    onStreamStart?: () => void;
    onStreamEnd?: () => void;
}

interface ChatMsg {
    role: 'user' | 'assistant';
    content: string;
    hasImage: boolean;
    element?: HTMLElement;
}

export class ChatUI {
    private messages: ChatMsg[] = [];
    private deps: ChatDeps;
    private isStreaming = false;
    private currentStreamEl: HTMLElement | null = null;
    private attachFrame = false;

    constructor(deps: ChatDeps) {
        this.deps = deps;
        this.deps.sendBtn.addEventListener('click', () => this.send());
        this.deps.clearBtn.addEventListener('click', () => this.clear());
        this.deps.captureBtn.addEventListener('click', () => this.toggleAttach());

        this.deps.input.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                this.send();
            }
        });

        // Keyboard shortcut: Ctrl+Shift+F to capture frame
        document.addEventListener('keydown', (e) => {
            if (e.ctrlKey && e.shiftKey && e.key === 'F') {
                e.preventDefault();
                this.toggleAttach();
            }
        });
    }

    private toggleAttach(): void {
        this.attachFrame = !this.attachFrame;
        this.deps.captureBtn.style.background = this.attachFrame
            ? 'rgba(249, 115, 22, 0.2)'
            : '';
        this.deps.captureBtn.style.color = this.attachFrame ? '#f97316' : '';
    }

    private async send(): Promise<void> {
        const text = this.deps.input.value.trim();
        if (!text || this.isStreaming) return;

        const frame = this.attachFrame ? this.deps.getFrame() : '';
        this.addMessage('user', text, !!frame);
        this.deps.input.value = '';
        this.attachFrame = false;
        this.deps.captureBtn.style.background = '';
        this.deps.captureBtn.style.color = '';
        this.setLoading(true);

        try {
            const reply = await this.deps.sendMessage(text, frame);
            // If streaming filled the bubble, don't duplicate
            if (!this.currentStreamEl) {
                this.addMessage('assistant', reply, false);
            }
        } catch (err: unknown) {
            const msg = err instanceof Error ? err.message : String(err);
            this.addMessage('assistant', `Error: ${msg}`, false);
        } finally {
            this.setLoading(false);
            this.currentStreamEl = null;
        }
    }

    private async clear(): Promise<void> {
        await this.deps.clearChat();
        this.messages = [];
        this.deps.container.innerHTML = '';
    }

    addMessage(role: 'user' | 'assistant', content: string, hasImage: boolean): HTMLElement {
        const msg: ChatMsg = { role, content, hasImage };
        this.messages.push(msg);

        const wrapper = document.createElement('div');
        wrapper.className = `message message-${role}`;

        const meta = document.createElement('div');
        meta.className = 'msg-meta';
        meta.textContent = role === 'user' ? 'YOU' : 'VISIONCHAT';

        const bubble = document.createElement('div');
        bubble.className = 'msg-bubble';

        if (hasImage) {
            const badge = document.createElement('div');
            badge.className = 'msg-frame-badge';
            badge.textContent = '📷 frame attached';
            bubble.appendChild(badge);
        }

        const textEl = document.createElement('div');
        textEl.textContent = content;
        bubble.appendChild(textEl);

        wrapper.appendChild(meta);
        wrapper.appendChild(bubble);
        this.deps.container.appendChild(wrapper);
        this.scrollToBottom();

        msg.element = wrapper;
        return wrapper;
    }

    // Called for each streaming token
    appendStreamToken(token: string): void {
        if (!this.currentStreamEl) {
            this.currentStreamEl = this.addMessage('assistant', '', false);
        }

        const bubble = this.currentStreamEl.querySelector('.msg-bubble div:last-child');
        if (bubble) {
            // Remove cursor if exists
            const cursor = bubble.querySelector('.cursor-blink');
            if (cursor) cursor.remove();

            bubble.textContent += token;

            // Add typing cursor
            const cursorEl = document.createElement('span');
            cursorEl.className = 'cursor-blink';
            bubble.appendChild(cursorEl);
        }
        this.scrollToBottom();
    }

    finishStream(): void {
        if (this.currentStreamEl) {
            const cursor = this.currentStreamEl.querySelector('.cursor-blink');
            if (cursor) cursor.remove();
        }
        this.currentStreamEl = null;
        this.setLoading(false);
    }

    private setLoading(loading: boolean): void {
        this.isStreaming = loading;
        this.deps.sendBtn.disabled = loading;
        this.deps.sendBtn.textContent = loading ? 'Thinking...' : 'Send ➤';
        this.deps.onStreamStart?.();
    }

    private scrollToBottom(): void {
        this.deps.container.scrollTop = this.deps.container.scrollHeight;
    }
}
