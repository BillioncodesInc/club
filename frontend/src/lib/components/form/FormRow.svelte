<script>
	export let label = '';
	export let required = false;
	export let description = '';
</script>

<div class="space-y-1">
	{#if label}
		<label class="block text-sm font-medium text-gray-700 dark:text-gray-300">
			{label}
			{#if required}<span class="text-red-500">*</span>{/if}
		</label>
	{/if}
	{#if description}
		<p class="text-xs text-gray-500 dark:text-gray-400">{description}</p>
	{/if}
	<slot />
</div>
