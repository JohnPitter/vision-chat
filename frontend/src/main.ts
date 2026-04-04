import './style.css';
import { VideoCapture, VideoSource } from './video';
import { ChatUI } from './chat';
import {
    SendMessage,
    ClearChat,
    AnalyzeFrame,
    StartAutoDescribe,
    StopAutoDescribe,
    AutoDescribeFrame,
    GetCacheStats,
    GetServerStatus,
    ConfirmTool,
    DenyTool,
} from '../wailsjs/go/main/App';
import { EventsOn } from '../wailsjs/runtime/runtime';

// === DOM refs ===
const videoEl = document.getElementById('webcam') as HTMLVideoElement;
const video = new VideoCapture(videoEl);
const stopBtn = document.getElementById('btn-stop') as HTMLButtonElement;
const sendBtn = document.getElementById('btn-send') as HTMLButtonElement;
const autoBtn = document.getElementById('btn-auto') as HTMLButtonElement;
const autoLed = document.getElementById('auto-led') as HTMLElement;
const autoLabel = document.getElementById('auto-label') as HTMLElement;
const autoBanner = document.getElementById('auto-banner') as HTMLElement;
const autoText = document.getElementById('auto-text') as HTMLElement;
const motionDot = document.getElementById('motion-dot') as HTMLElement;
const hudLed = document.getElementById('hud-led') as HTMLElement;
const hudFps = document.getElementById('hud-fps') as HTMLElement;
const hudCache = document.getElementById('hud-cache') as HTMLElement;
const hudInterval = document.getElementById('hud-interval') as HTMLElement;
const statusEl = document.getElementById('server-status') as HTMLElement;
const statusText = document.getElementById('status-text') as HTMLElement;
const statTotal = document.getElementById('stat-total') as HTMLElement;
const statCached = document.getElementById('stat-cached') as HTMLElement;
const statRate = document.getElementById('stat-rate') as HTMLElement;
const statInterval = document.getElementById('stat-interval') as HTMLElement;

// Source selector buttons
const btnWebcam = document.getElementById('btn-webcam') as HTMLButtonElement;
const btnScreen = document.getElementById('btn-screen') as HTMLButtonElement;
const btnWindow = document.getElementById('btn-window') as HTMLButtonElement;
const sourceBtns = [btnWebcam, btnScreen, btnWindow];

// === Chat UI ===
const chatUI = new ChatUI({
    container: document.getElementById('chat-messages')!,
    input: document.getElementById('chat-input') as HTMLTextAreaElement,
    sendBtn,
    clearBtn: document.getElementById('btn-clear') as HTMLButtonElement,
    captureBtn: document.getElementById('btn-capture') as HTMLButtonElement,
    getFrame: () => (video.isCapturing ? video.captureFrame() : ''),
    sendMessage: SendMessage,
    clearChat: ClearChat,
});

// === Video Source Selection ===
function setActiveSource(btn: HTMLButtonElement): void {
    sourceBtns.forEach((b) => b.classList.remove('source-active'));
    btn.classList.add('source-active');
}

async function startSource(source: VideoSource, btn: HTMLButtonElement): Promise<void> {
    try {
        // Stop existing capture
        if (video.isCapturing) {
            video.stop();
            if (_captureLoopId !== null) {
                cancelAnimationFrame(_captureLoopId);
                _captureLoopId = null;
            }
        }

        await video.start(source);
        setActiveSource(btn);
        stopBtn.disabled = false;

        // Mirror only for webcam
        if (source === 'webcam') {
            videoEl.classList.add('mirror');
        } else {
            videoEl.classList.remove('mirror');
        }

        startCaptureLoop();
    } catch (err) {
        console.error('Video source error:', err);
    }
}

btnWebcam.addEventListener('click', () => startSource('webcam', btnWebcam));
btnScreen.addEventListener('click', () => startSource('screen', btnScreen));
btnWindow.addEventListener('click', () => startSource('window', btnWindow));

// Handle stop
stopBtn.addEventListener('click', () => {
    video.stop();
    if (_captureLoopId !== null) {
        cancelAnimationFrame(_captureLoopId);
        _captureLoopId = null;
    }
    stopBtn.disabled = true;
    videoEl.classList.remove('mirror');
});

// Handle browser-initiated stop (user clicks "Stop sharing" in browser UI)
window.addEventListener('videocapture:stopped', () => {
    if (_captureLoopId !== null) {
        cancelAnimationFrame(_captureLoopId);
        _captureLoopId = null;
    }
    stopBtn.disabled = true;
    videoEl.classList.remove('mirror');
});

// === Frame Capture Loop (60fps) ===
let _captureLoopId: number | null = null;
let lastAnalyzeTime = 0;

function startCaptureLoop(): void {
    function loop(): void {
        if (!video.isCapturing) {
            _captureLoopId = null;
            return;
        }

        const now = performance.now();
        const frame = video.captureFrame();

        // Analyze frame for motion detection at adaptive intervals
        if (now - lastAnalyzeTime >= 16) {
            lastAnalyzeTime = now;
            AnalyzeFrame(frame)
                .then((result: Record<string, unknown>) => {
                    const isNew = result['isNew'] as boolean;
                    const changePercent = result['changePercent'] as number;

                    if (isNew && changePercent > 0.03) {
                        motionDot.className = 'led led-pulse';
                    } else {
                        motionDot.className = 'led led-idle';
                    }
                })
                .catch(() => {});
        }

        hudFps.textContent = video.fps.toFixed(1);
        _captureLoopId = requestAnimationFrame(loop);
    }
    _captureLoopId = requestAnimationFrame(loop);
}

// === Telemetry Update (every 1s) ===
setInterval(async () => {
    try {
        const stats = (await GetCacheStats()) as Record<string, unknown>;
        const total = stats['totalFrames'] as number;
        const cached = stats['cachedFrames'] as number;
        const rate = stats['savedPercent'] as number;
        const interval = stats['currentInterval'] as number;

        hudCache.textContent = `${rate.toFixed(1)}%`;
        hudInterval.textContent = `${interval}ms`;

        statTotal.textContent = total.toLocaleString();
        statCached.textContent = cached.toLocaleString();
        statRate.textContent = `${rate.toFixed(1)}%`;
        statInterval.textContent = `${interval}ms`;
    } catch (_) {}
}, 1000);

// === Auto-Describe ===
let autoDescActive = false;

autoBtn.addEventListener('click', async () => {
    autoDescActive = !autoDescActive;
    if (autoDescActive) {
        await StartAutoDescribe(3000);
        autoLed.className = 'led led-pulse';
        autoLabel.textContent = 'AUTO-DESCRIBE ON';
        autoBanner.classList.remove('hidden');
        autoText.textContent = 'Waiting for scene change...';
    } else {
        await StopAutoDescribe();
        autoLed.className = 'led led-idle';
        autoLabel.textContent = 'AUTO-DESCRIBE OFF';
        autoBanner.classList.add('hidden');
    }
});

EventsOn('auto:request-frame', () => {
    if (video.isCapturing) {
        autoText.textContent = ''; // clear before new description
        const frame = video.captureFrame();
        AutoDescribeFrame(frame);
    }
});

EventsOn('auto:stream', (token: string) => {
    autoText.textContent += token;
});

EventsOn('auto:done', (text: string) => {
    autoText.textContent = text;
});

// === Streaming ===
EventsOn('chat:stream', (token: string) => {
    chatUI.appendStreamToken(token);
});

EventsOn('chat:stream:done', () => {
    chatUI.finishStream();
});

// === Server Status ===
function setServerReady(): void {
    statusEl.className = 'status-badge status-ready';
    statusText.textContent = 'Model Ready';
    hudLed.className = 'led led-ok';
    sendBtn.disabled = false;
}

EventsOn('server:ready', () => setServerReady());

EventsOn('server:error', (err: string) => {
    statusEl.className = 'status-badge status-error';
    statusText.textContent = `Error: ${err}`;
    hudLed.className = 'led led-error';
});

// Fallback: poll server status in case the event was missed
const statusPoll = setInterval(async () => {
    try {
        const status = await GetServerStatus();
        if (status === 'running' || status === 'stopped') {
            // Server manager knows about it, but also check if client works
            // by trying GetCacheStats (which requires the client to be set)
            await GetCacheStats();
            setServerReady();
            clearInterval(statusPoll);
        }
    } catch (_) {}
}, 1000);

// === Keyboard Shortcuts ===
document.addEventListener('keydown', (e) => {
    if (e.ctrlKey && e.shiftKey && e.key === 'A') {
        e.preventDefault();
        autoBtn.click();
    }
    // Ctrl+1/2/3 to switch sources
    if (e.ctrlKey && e.key === '1') {
        e.preventDefault();
        btnWebcam.click();
    }
    if (e.ctrlKey && e.key === '2') {
        e.preventDefault();
        btnScreen.click();
    }
    if (e.ctrlKey && e.key === '3') {
        e.preventDefault();
        btnWindow.click();
    }
});

// === Tool Confirmation Dialog ===
const toolDialog = document.getElementById('tool-dialog') as HTMLElement;
const toolDialogText = document.getElementById('tool-dialog-text') as HTMLElement;
const btnToolConfirm = document.getElementById('btn-tool-confirm') as HTMLButtonElement;
const btnToolDeny = document.getElementById('btn-tool-deny') as HTMLButtonElement;

EventsOn('tool:confirm', (data: Record<string, unknown>) => {
    const name = data['name'] as string;
    const args = data['args'] as Record<string, string>;
    const argsStr = Object.entries(args).map(([k, v]) => `  ${k}: ${v}`).join('\n');
    toolDialogText.textContent = `Tool: ${name}\nArguments:\n${argsStr}`;
    toolDialog.classList.remove('hidden');
});

btnToolConfirm.addEventListener('click', () => {
    toolDialog.classList.add('hidden');
    ConfirmTool();
});

btnToolDeny.addEventListener('click', () => {
    toolDialog.classList.add('hidden');
    DenyTool();
});

EventsOn('tool:result', (data: Record<string, unknown>) => {
    const name = data['name'] as string;
    const success = data['success'] as boolean;
    const output = (data['output'] || data['error']) as string;
    const status = success ? 'OK' : 'FAILED';
    chatUI.addMessage('assistant', `[Tool ${name}: ${status}] ${output}`, false);
});
