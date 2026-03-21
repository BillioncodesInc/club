<script>
	export let title = '';
	export let subtitle = '';
</script>
<h1
	class="text-3xl text-gray-600 dark:text-gray-300 font-bold uppercase transition-colors duration-200"
>
	{#if title}
		{title}
	{:else}
		<slot />
	{/if}
</h1>
{#if subtitle}
	<p class="text-sm text-gray-500 dark:text-gray-400 mt-1 transition-colors duration-200">{subtitle}</p>
{/if}
