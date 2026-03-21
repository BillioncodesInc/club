<script>
	export let title = '';
</script>
<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-6 transition-colors duration-200">
	{#if title}
		<h3 class="text-lg font-semibold text-gray-700 dark:text-gray-200 mb-4 transition-colors duration-200">{title}</h3>
	{/if}
	<slot />
</div>
