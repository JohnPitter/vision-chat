export type VideoSource = 'webcam' | 'screen' | 'window';

export class VideoCapture {
    private video: HTMLVideoElement;
    private canvas: HTMLCanvasElement;
    private ctx: CanvasRenderingContext2D;
    private stream: MediaStream | null = null;
    private _isCapturing = false;
    private _source: VideoSource = 'webcam';
    private _trackLabel = '';
    private frameCount = 0;
    private lastFpsTime = 0;
    private _fps = 0;

    constructor(videoElement: HTMLVideoElement) {
        this.video = videoElement;
        this.canvas = document.createElement('canvas');
        this.ctx = this.canvas.getContext('2d')!;
    }

    async start(source: VideoSource = 'webcam'): Promise<void> {
        // Stop any existing stream first
        this.stopStream();

        this._source = source;

        if (source === 'webcam') {
            this.stream = await navigator.mediaDevices.getUserMedia({
                video: { width: 640, height: 480, frameRate: 60, facingMode: 'user' },
            });
        } else {
            // screen or window - use getDisplayMedia
            const displayMediaOptions: DisplayMediaStreamOptions = {
                video: {
                    frameRate: 60,
                },
                audio: false,
            };

            // For 'window' mode, the browser picker lets user choose a window
            // For 'screen' mode, the browser picker lets user choose entire screen
            // The browser handles this distinction in its native picker UI
            this.stream = await navigator.mediaDevices.getDisplayMedia(displayMediaOptions);

            // Capture the track label (contains window/screen name)
            const track = this.stream.getVideoTracks()[0];
            this._trackLabel = track.label || '';

            // Listen for when user stops sharing via browser UI
            track.addEventListener('ended', () => {
                this._isCapturing = false;
                this.video.srcObject = null;
                this.stream = null;
                this._fps = 0;
                // Dispatch custom event so main.ts can update UI
                window.dispatchEvent(new CustomEvent('videocapture:stopped'));
            });
        }

        this.video.srcObject = this.stream;
        await this.video.play();
        this._isCapturing = true;
        this.lastFpsTime = performance.now();
        this.frameCount = 0;
    }

    private stopStream(): void {
        if (this.stream) {
            this.stream.getTracks().forEach((t) => t.stop());
            this.stream = null;
        }
    }

    stop(): void {
        this.stopStream();
        this.video.srcObject = null;
        this._isCapturing = false;
        this._fps = 0;
    }

    captureFrame(): string {
        this.canvas.width = this.video.videoWidth;
        this.canvas.height = this.video.videoHeight;
        this.ctx.drawImage(this.video, 0, 0);

        // Update FPS counter
        this.frameCount++;
        const now = performance.now();
        const elapsed = now - this.lastFpsTime;
        if (elapsed >= 1000) {
            this._fps = (this.frameCount / elapsed) * 1000;
            this.frameCount = 0;
            this.lastFpsTime = now;
        }

        const dataURL = this.canvas.toDataURL('image/jpeg', 0.7);
        return dataURL.split(',')[1]; // raw base64 without prefix
    }

    get isCapturing(): boolean {
        return this._isCapturing;
    }

    get fps(): number {
        return this._fps;
    }

    get source(): VideoSource {
        return this._source;
    }

    get trackLabel(): string {
        return this._trackLabel;
    }
}
