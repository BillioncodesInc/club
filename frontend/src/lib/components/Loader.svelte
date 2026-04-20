<script>
	import { isLoading } from '$lib/store/loading.js';
	import { blur } from 'svelte/transition';

	const duration = 250; //ms

	// throttle the animation flag so quick loading blips don't flash the overlay
	let isAnimating = false;
	let throttleId;
	$: {
		clearTimeout(throttleId);
		throttleId = setTimeout(() => {
			isAnimating = $isLoading;
		}, 150);
	}
</script>

{#if $isLoading && isAnimating}
	<div class="fixed top-0 left-0 w-full h-full opacity-[0.5]" transition:blur={{ duration }} />
	<div
		transition:blur={{ duration }}
		class="fixed top-0 left-0 w-full h-full flex justify-center items-center backdrop-blur-sm z-50"
	>
		<div
			class="w-20 h-20 border-t-8 border-t-cta-blue border-r-8 border-r-cta-blue border-b-cta-blue border-b-8 border-l-transparent border-l-8 rounded-full animate-spin"
		></div>
	</div>
{/if}
