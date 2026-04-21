<script>
	/**
	 * Unified Button primitive.
	 *
	 * Preferred API (Phase 5):
	 *   <Button variant="primary" size="md" on:click={...}>Label</Button>
	 *
	 * Legacy API (still supported — old call sites pass these):
	 *   backgroundColor: raw Tailwind bg class (e.g. 'bg-cta-blue')
	 *   css:             extra classes appended verbatim
	 *   size:            'small' | 'medium' | 'large' — mapped to sm|md|lg
	 *
	 * Public event: click (forwarded).
	 * Public slot:  default.
	 */

	/** @type {'primary'|'secondary'|'danger'|'ghost'|'outline'} */
	export let variant = 'primary';
	/** @type {'sm'|'md'|'lg'|'small'|'medium'|'large'} */
	export let size = 'md';
	/** @type {'button'|'submit'|'reset'} */
	export let type = 'button';
	export let disabled = false;

	// legacy escape hatches — prefer `variant` / `size`
	export let backgroundColor = '';
	export let css = '';

	// normalize legacy size names
	const sizeAliases = { small: 'sm', medium: 'md', large: 'lg' };
	$: normalizedSize = sizeAliases[size] || size;

	const variantClasses = {
		primary: 'bg-pc-pink text-white hover:opacity-90 active:opacity-80',
		secondary:
			'bg-gray-200 text-gray-900 dark:bg-gray-700 dark:text-white hover:bg-gray-300 dark:hover:bg-gray-600',
		danger: 'bg-red-600 text-white hover:bg-red-700',
		ghost:
			'bg-transparent text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800',
		outline:
			'bg-transparent border border-gray-300 text-gray-700 dark:border-gray-600 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-800'
	};

	const sizeClasses = {
		sm: 'px-3 py-1.5 text-sm',
		md: 'px-4 py-2 text-base',
		lg: 'px-6 py-3 text-lg'
	};

	const commonClasses =
		'rounded-md font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-pc-pink focus-visible:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed';

	// if the caller supplied a legacy backgroundColor, let it win over the variant bg.
	$: variantClass = backgroundColor ? '' : variantClasses[variant] || variantClasses.primary;
	$: sizeClass = sizeClasses[normalizedSize] || sizeClasses.md;
</script>

<button
	{type}
	{disabled}
	on:click
	class="{commonClasses} {variantClass} {sizeClass} {backgroundColor} {css}"
>
	<slot />
</button>
