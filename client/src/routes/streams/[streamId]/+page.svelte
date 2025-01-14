<script lang="ts">
	import { onMount } from 'svelte';
	import Hls from 'hls.js';
	import { page } from '$app/state';

	let videoElement: HTMLVideoElement;
	let errorMessage = '';
	let showError = false;

	// get the streamId from the URL params
	$: streamId = page.params.streamId;

	onMount(() => {
		initPlayer();
	});

	function initPlayer() {
		if (!videoElement) return;

		const streamUrl = `http://localhost:8080/hls/${streamId}.m3u8`;

		// Check for native HLS support (Safari)
		if (videoElement.canPlayType('application/vnd.apple.mpegurl')) {
			videoElement.src = streamUrl;
			videoElement.addEventListener('error', () => {
				showError = true;
				errorMessage = 'Stream playback error';
			});
		}
		// Use HLS.js if supported
		else if (Hls.isSupported()) {
			const hls = new Hls({
				debug: false,
				enableWorker: true
			});

			hls.loadSource(streamUrl);
			hls.attachMedia(videoElement);

			hls.on(Hls.Events.ERROR, (event, data) => {
				if (data.fatal) {
					showError = true;
					errorMessage = 'HLS playback error';
					console.error('HLS error:', data);
				}
			});
		}
		// No HLS support
		else {
			showError = true;
			errorMessage = 'Your browser does not support HLS playback.';
		}
	}
</script>

<div class="flex min-h-screen items-center justify-center bg-gray-100">
	<div class="w-[95%] max-w-3xl rounded-lg bg-white p-5 shadow">
		<video bind:this={videoElement} controls class="w-full rounded">
			<track kind="captions" />
		</video>

		{#if showError}
			<div class="mt-2 text-red-500">
				{errorMessage}
			</div>
		{/if}
	</div>
</div>

<style>
	:global(body) {
		margin: 0;
		font-family: Arial, sans-serif;
	}
</style>
