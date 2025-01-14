<script lang="ts">
	import '../app.css';
	let { children } = $props();
</script>

<main class="h-screen bg-neutral-200">
	{@render children()}
</main>
