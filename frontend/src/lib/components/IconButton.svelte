<script>
	import {
		Pencil,
		X,
		Trash2,
		Copy,
		Download,
		Upload,
		Eye,
		UserX
	} from 'lucide-svelte';

	export let variant = 'blue'; // blue, green, orange, red, gray
	export let icon = 'edit'; // edit, close, anonymize, export, upload, delete, copy, view
	export let disabled = false;
	export let type = 'button';

	// color mappings for variants
	const variantClasses = {
		blue: 'bg-blue-600 hover:bg-blue-700 focus:ring-blue-500',
		green: 'bg-green-700 hover:bg-green-800 focus:ring-green-600',
		orange: 'bg-orange-600 hover:bg-orange-700 focus:ring-orange-500',
		red: 'bg-red-600 hover:bg-red-700 focus:ring-red-500',
		gray: 'bg-gray-600 hover:bg-gray-700 focus:ring-gray-500'
	};

	$: colorClass = variantClasses[variant] || variantClasses.blue;

	// map the legacy icon prop values to lucide components
	const iconMap = {
		edit: Pencil,
		close: X,
		delete: Trash2,
		copy: Copy,
		export: Download,
		upload: Upload,
		view: Eye,
		anonymize: UserX
	};

	$: IconComponent = iconMap[icon] || null;
</script>

<button
	{type}
	{disabled}
	on:click
	class="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white {colorClass} focus:outline-none focus:ring-2 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed"
>
	{#if IconComponent}
		<svelte:component this={IconComponent} class="h-4 w-4 mr-2" strokeWidth={2} />
	{/if}
	<slot />
</button>
