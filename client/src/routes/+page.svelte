<script lang="ts">
	import Button from '$lib/components/ui/button/button.svelte';
	import Input from '@/components/ui/input/input.svelte';

	let isStreaming = $state(false);

	let stream: MediaStream | undefined = $state();
	let videoElement: HTMLVideoElement;
	let mediaRecorder: MediaRecorder;
	let socket: WebSocket;

	async function startStream() {
		try {
			// Get webcam stream
			stream = await navigator.mediaDevices.getUserMedia({
				video: true,
				audio: true
			});

			videoElement.srcObject = stream;

			// Setup WebSocket
			socket = new WebSocket('ws://localhost:5000/ws');

			socket.onopen = () => {
				isStreaming = true;
			};

			socket.onclose = () => {
				isStreaming = false;
			};

			// Create MediaRecorder
			mediaRecorder = new MediaRecorder(stream, {
				mimeType: 'video/webm;codecs=vp8,opus'
			});

			// Send chunks to server
			mediaRecorder.ondataavailable = (event) => {
				if (event.data.size > 0 && socket.readyState === WebSocket.OPEN) {
					socket.send(event.data);
				}
			};

			// Start recording
			mediaRecorder.start(25); // Create chunks every 1 second
		} catch (err) {
			console.error('Error starting stream:', err);
		}
	}

	function closeStream() {
		mediaRecorder.stop();
		stream?.getTracks().forEach((track) => track.stop());
		socket.close();
	}
</script>

<div class="flex flex-col items-center justify-center gap-y-4 pt-8">
	<h1 class="text-3xl font-bold">Live Stream</h1>
	{#if !isStreaming}
		<Button onclick={startStream}>Start Stream</Button>
		<Input />
	{/if}
	{#if isStreaming}
		<Button onclick={closeStream}>Close Stream</Button>
	{/if}
	<video class="rounded-lg" bind:this={videoElement} id="localVideo" autoplay playsinline>
		<track kind="captions" />
	</video>

	{#if stream}
		<a href="/streams/stream" target="_blank">
			<Button onclick={startStream}>View Your Stream</Button>
		</a>
	{/if}
</div>
