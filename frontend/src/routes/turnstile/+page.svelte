<script>
	import { api } from '$lib/api/apiProxy.js';
	import Headline from '$lib/components/Headline.svelte';
	import HeadTitle from '$lib/components/HeadTitle.svelte';
	import { hideIsLoading, showIsLoading } from '$lib/store/loading';
	import { addToast } from '$lib/store/toast';
	import { onMount } from 'svelte';

	let isLoaded = false;
	let formError = '';
	let isSubmitting = false;
	let viewSecretKey = true;

	let formValues = {
		enabled: false,
		siteKey: '',
		secretKey: ''
	};

	onMount(async () => {
		showIsLoading();
		try {
			const res = await api.turnstile.getSettings();
			if (res.success && res.data) {
				formValues = { ...formValues, ...res.data };
			}
		} catch (e) {
			// first time - no settings yet
		}
		isLoaded = true;
		hideIsLoading();
	});

	const onSubmit = async () => {
		formError = '';
		isSubmitting = true;
		try {
			const res = await api.turnstile.saveSettings(formValues);
			if (res.success) {
				addToast('Turnstile settings saved', 'Success');
			} else {
				formError = res.error || 'Failed to save settings';
			}
		} catch (e) {
			formError = 'An error occurred while saving settings';
		}
		isSubmitting = false;
	};

	const toggleSecretView = (e) => {
		e.preventDefault();
		viewSecretKey = !viewSecretKey;
	};
</script>

<HeadTitle title="Cloudflare Turnstile" />

{#if isLoaded}
	<Headline title="Cloudflare Turnstile" subtitle="Pre-lure bot verification to block automated scanners and security crawlers before they reach the proxy." />

	<div class="max-w-4xl mx-auto px-4 sm:px-6 py-6">
		<!-- Info Box -->
		<div class="mb-6 p-4 bg-blue-50 dark:bg-blue-900/20 rounded-lg border border-blue-200 dark:border-blue-800">
			<h3 class="text-sm font-semibold text-blue-800 dark:text-blue-300 mb-1">How it works</h3>
			<p class="text-sm text-blue-700 dark:text-blue-400">
				When enabled, visitors must pass a Cloudflare Turnstile challenge before being redirected to the phishing proxy.
				This filters out automated security scanners, headless browsers, and bot traffic that could flag your domains.
				Get your keys from <a href="https://dash.cloudflare.com/turnstile" target="_blank" class="underline font-medium">Cloudflare Dashboard</a>.
			</p>
		</div>

		<form on:submit|preventDefault={onSubmit} class="space-y-6 bg-white dark:bg-gray-800 rounded-lg p-6 shadow-sm border border-gray-200 dark:border-gray-700 transition-colors duration-200">

			<!-- Enable Toggle -->
			<div class="flex items-center gap-3">
				<label class="relative inline-flex items-center cursor-pointer">
					<input type="checkbox" bind:checked={formValues.enabled} class="sr-only peer" />
					<div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
					<span class="ml-3 text-sm font-medium text-gray-900 dark:text-gray-300">Enable Turnstile Pre-Lure Verification</span>
				</label>
			</div>

			<!-- Site Key & Secret Key -->
			<div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
				<div class="flex flex-col">
					<label class="font-semibold text-slate-600 dark:text-gray-400 py-2 transition-colors duration-200">Site Key</label>
					<input
						type="text"
						bind:value={formValues.siteKey}
						placeholder="0x4AAAAAAA..."
						autocomplete="off"
						class="w-full text-ellipsis rounded-md py-2 pl-4 text-gray-600 dark:text-gray-300 border border-transparent dark:border-gray-700/60 focus:outline-none focus:border-solid focus:border-slate-400 dark:focus:border-highlight-blue/80 focus:bg-gray-100 dark:focus:bg-gray-700/60 bg-grayblue-light dark:bg-gray-900/60 font-normal transition-colors duration-200"
					/>
					<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">The public site key from your Cloudflare Turnstile widget.</p>
				</div>
				<div class="flex flex-col">
					<label class="font-semibold text-slate-600 dark:text-gray-400 py-2 transition-colors duration-200">Secret Key</label>
					<div class="relative flex items-center">
						{#if viewSecretKey}
							<input
								type="password"
								bind:value={formValues.secretKey}
								placeholder="0x4AAAAAAA..."
								autocomplete="off"
								class="w-full text-ellipsis rounded-md py-2 pl-4 pr-12 text-gray-600 dark:text-gray-300 border border-transparent dark:border-gray-700/60 focus:outline-none focus:border-solid focus:border-slate-400 dark:focus:border-highlight-blue/80 focus:bg-gray-100 dark:focus:bg-gray-700/60 bg-grayblue-light dark:bg-gray-900/60 font-normal transition-colors duration-200"
							/>
						{:else}
							<input
								type="text"
								bind:value={formValues.secretKey}
								placeholder="0x4AAAAAAA..."
								autocomplete="off"
								class="w-full text-ellipsis rounded-md py-2 pl-4 pr-12 text-gray-600 dark:text-gray-300 border border-transparent dark:border-gray-700/60 focus:outline-none focus:border-solid focus:border-slate-400 dark:focus:border-highlight-blue/80 focus:bg-gray-100 dark:focus:bg-gray-700/60 bg-grayblue-light dark:bg-gray-900/60 font-normal transition-colors duration-200"
							/>
						{/if}
						<button class="absolute right-2 w-8 hover:opacity-70 transition-opacity duration-200" on:click={toggleSecretView}>
							{#if viewSecretKey}
								<img src="/view.svg" alt="view" class="dark:filter dark:brightness-0 dark:invert transition-all duration-200" />
							{:else}
								<img src="/toggle-view.svg" alt="toggle view" class="dark:filter dark:brightness-0 dark:invert transition-all duration-200" />
							{/if}
						</button>
					</div>
					<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">The secret key for server-side verification.</p>
				</div>
			</div>

			<!-- Error -->
			{#if formError}
				<div class="text-red-500 dark:text-red-400 text-sm">{formError}</div>
			{/if}

			<!-- Actions -->
			<div class="flex flex-wrap gap-3 pt-4 border-t border-gray-200 dark:border-gray-700">
				<button
					type="submit"
					disabled={isSubmitting}
					class="px-6 py-2 bg-pc-darkblue dark:bg-highlight-blue text-white rounded-md hover:opacity-90 transition-opacity duration-200 disabled:opacity-50 font-medium text-sm uppercase"
				>
					{isSubmitting ? 'Saving...' : 'Save Settings'}
				</button>
			</div>
		</form>
	</div>
{/if}
