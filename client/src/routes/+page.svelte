<script lang="ts">
	import Button from '$lib/components/ui/button/button.svelte';
	import Badge from '@/components/ui/badge/badge.svelte';
	import Input from '@/components/ui/input/input.svelte';
	import Label from '@/components/ui/label/label.svelte';
	import Textarea from '@/components/ui/textarea/textarea.svelte';
	import { startStreamSchema, type TStartStreamSchema } from '@/schema';
	import type { ZodError } from 'zod';

	let isStreaming = $state(false);
	let formData = $state<TStartStreamSchema>({
		title: '',
		description: '',
		tags: ['Gaming', 'Programming', 'Learning']
	});

	let formErrors: ZodError<TStartStreamSchema>['formErrors'] | undefined = $state();
	let streamErrors: string | undefined = $state();

	let stream: MediaStream | undefined = $state();
	let videoElement: HTMLVideoElement | null = null;
	let mediaRecorder: MediaRecorder | undefined;
	let socket: WebSocket | undefined;

	// First, let's extend the state to track form submission attempts
	let isSubmitted = $state(false);
	let isValid = $state(false);

	// Validation function
	function validateForm() {
		let result = startStreamSchema.safeParse(formData);

		if (result.success) {
			formErrors = undefined;
			isValid = true;
		} else {
			result.error;
			formErrors = result.error.formErrors;
			isValid = false;
		}
		return isValid;
	}

	// Start Stream function
	async function startStream() {
		try {
			// Get webcam stream
			stream = await navigator.mediaDevices.getUserMedia({
				video: true,
				audio: true
			});

			if (videoElement) {
				videoElement.srcObject = stream;
			}

			// Setup WebSocket
			socket = new WebSocket('ws://localhost:5000/ws');

			// start the stream when connection is open
			socket.onopen = () => {
				// fetch user id from the session when we implement the auth
				socket?.send(
					JSON.stringify({
						type: 'start',
						data: {
							channelName: 'dilshad' + Math.random(),
							title: formData.title,
							description: formData.description,
							tags: ['gaming', 'learning']
						}
					})
				);
				isStreaming = true;
			};

			socket.onmessage = (e) => {
				console.log('message from the server', e.data);
				const data = JSON.parse(e.data) as { type: string; data: any };

				switch (data.type) {
					case 'stream-start':
						// console.log('stream data', data.data);
						if (data.data === 'start') {
							streamErrors = undefined;
							isStreaming = true;
						} else {
							streamErrors = 'An error occurred while starting the stream';
						}
						break;
					case 'stop':
						closeStream();
						break;
					default:
						break;
				}
			};

			socket.onclose = () => {
				isStreaming = false;
			};

			// Create MediaRecorder
			mediaRecorder = new MediaRecorder(stream, {
				mimeType: 'video/webm',
				bitsPerSecond: 2500000, // Increased bitrate
				videoBitsPerSecond: 2000000,
				audioBitsPerSecond: 500000
			});

			// Send chunks to server
			mediaRecorder.ondataavailable = (event) => {
				// console.log(isStreaming);
				if (event.data.size > 0 && socket?.readyState === WebSocket.OPEN) {
					console.log('data to send', event.data);
					// socket?.send(JSON.stringify({ type: 'stream', data: event.data }));
					socket.send(event.data);
				}
			};

			// Start recording
			mediaRecorder.start(200); // Create chunks every 1 second
		} catch (err) {
			console.log((err as Error).name);
			let error = err as Error;
			console.log(error);
			if (error.name === 'InvalidStateError') {
				streamErrors = 'An error occurred while starting the stream';
			}

			if (videoElement) videoElement.srcObject = null;
		}
	}

	// Modified startStream function
	async function handleSubmit(e: Event) {
		e.preventDefault();
		console.log(validateForm());
		isSubmitted = true;

		if (!validateForm()) {
			console.log(formErrors);
			return;
		}

		await startStream();
	}

	function closeStream() {
		mediaRecorder?.stop();
		stream?.getTracks().forEach((track) => track.stop());
		socket?.close();
	}
</script>

<div class="mx-auto flex w-1/3 flex-col justify-center gap-y-4 pt-8">
	<h1 class="text-3xl font-bold">Live Stream</h1>
	{#if !isStreaming}
		<form onsubmit={handleSubmit} class="space-y-4">
			<div class="flex flex-col gap-2">
				<Label for="title">Title</Label>
				<Input
					type="text"
					id="title"
					name="title"
					bind:value={formData.title}
					oninput={() => isSubmitted && validateForm()}
				/>
				{#if isSubmitted && formErrors?.fieldErrors?.title}
					<span class="text-sm text-red-500">{formErrors.fieldErrors.title[0]}</span>
				{/if}
			</div>

			<div class="flex flex-col gap-2">
				<Label for="description">Description</Label>
				<Textarea
					id="description"
					name="description"
					bind:value={formData.description}
					oninput={() => isSubmitted && validateForm()}
				/>
				{#if isSubmitted && formErrors?.fieldErrors?.description}
					<span class="text-sm text-red-500">{formErrors.fieldErrors.description[0]}</span>
				{/if}
			</div>

			<div class="flex items-center gap-2">
				<Label for="tags">Tags</Label>
				<div class="flex items-center gap-2">
					<Badge variant="secondary">Learning</Badge>
					<Badge variant="secondary">Gaming</Badge>
					<Badge variant="secondary">Programming</Badge>
				</div>
			</div>

			<Button type="submit">Start Stream</Button>
			<p class="text-sm text-red-500">{streamErrors}</p>
		</form>
	{/if}
	<video class="rounded-lg" bind:this={videoElement} id="localVideo" autoplay playsinline>
		<track kind="captions" />
	</video>
	{#if isStreaming}
		<Button onclick={closeStream}>Close Stream</Button>
		<a href="/streams/stream" target="_blank">
			<Button>View Your Stream</Button>
		</a>
	{/if}
</div>
