<script>
	export let value = '';
	export let placeholder = '';
	export let rows = 4;
	export let disabled = false;
</script>

<textarea
	bind:value
	{placeholder}
	{rows}
	{disabled}
	class="w-full px-3 py-2 text-sm rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none transition-colors font-mono"
></textarea>
