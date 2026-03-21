<script>
	import { onMount } from 'svelte';
	import { api } from '$lib/api/apiProxy.js';
	import HeadTitle from '$lib/components/HeadTitle.svelte';
	import Headline from '$lib/components/Headline.svelte';
	import { hideIsLoading, showIsLoading } from '$lib/store/loading';
	import { addToast } from '$lib/store/toast';

	let config = {
		enabled: false,
		jsChallenge: true,
		behaviorAnalysis: true,
		fingerprintCheck: true,
		minInteractionTime: 2000,
		maxRequestRate: 30,
		challengeDifficulty: 'medium',
		blockHeadless: true,
		blockTor: false,
		blockVPN: false,
		whitelistedIPs: ''
	};

	let stats = {
		totalSessions: 0,
		passedSessions: 0,
		blockedSessions: 0,
		challengesSent: 0,
		challengesPassed: 0,
		challengesFailed: 0
	};

	let isLoaded = false;
	let saving = false;

	onMount(async () => {
		showIsLoading();
		await loadConfig();
		await loadStats();
		isLoaded = true;
		hideIsLoading();
	});

	async function loadConfig() {
		try {
			const res = await api.botGuard.getConfig();
			if (res && res.data) {
				config = { ...config, ...res.data };
			}
		} catch (e) {
			// first time - defaults are fine
		}
	}

	async function loadStats() {
		try {
			const res = await api.botGuard.getStats();
			if (res && res.data) {
				stats = { ...stats, ...res.data };
			}
		} catch (e) {
			// stats may not be available yet
		}
	}

	async function saveConfig() {
		saving = true;
		try {
			await api.botGuard.updateConfig(config);
			addToast('Bot Guard configuration saved', 'Success');
		} catch (e) {
			addToast('Failed to save configuration', 'Error');
		}
		saving = false;
	}

	async function cleanup() {
		try {
			await api.botGuard.cleanup();
			await loadStats();
			addToast('Expired sessions cleaned up', 'Success');
		} catch (e) {
			addToast('Cleanup failed', 'Error');
		}
	}
</script>

<HeadTitle title="Bot Guard" />

{#if isLoaded}
	<Headline title="Bot Guard" subtitle="Comprehensive bot detection with JavaScript challenges, behavior analysis, and browser fingerprinting." />

	<!-- Stats Cards -->
	<div class="grid grid-cols-3 gap-4 mb-8 mt-6">
		<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-4 text-center transition-colors duration-200">
			<div class="text-3xl font-bold text-blue-600 dark:text-blue-400">{stats.totalSessions}</div>
			<div class="text-sm text-gray-500 dark:text-gray-400">Total Sessions</div>
		</div>
		<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-4 text-center transition-colors duration-200">
			<div class="text-3xl font-bold text-green-600 dark:text-green-400">{stats.passedSessions}</div>
			<div class="text-sm text-gray-500 dark:text-gray-400">Passed</div>
		</div>
		<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-4 text-center transition-colors duration-200">
			<div class="text-3xl font-bold text-red-600 dark:text-red-400">{stats.blockedSessions}</div>
			<div class="text-sm text-gray-500 dark:text-gray-400">Blocked</div>
		</div>
	</div>

	<!-- Configuration -->
	<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-6 mb-6 transition-colors duration-200">
		<h2 class="text-lg font-semibold text-gray-700 dark:text-gray-200 mb-4">Configuration</h2>

		<div class="space-y-4">
			<label class="flex items-center gap-3">
				<input type="checkbox" bind:checked={config.enabled} class="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500" />
				<span class="font-medium text-gray-900 dark:text-gray-200">Enable Bot Guard</span>
			</label>

			<div class="border-t border-gray-200 dark:border-gray-700 pt-4">
				<h3 class="font-medium text-gray-800 dark:text-gray-200 mb-3">Detection Methods</h3>
				<div class="grid grid-cols-2 gap-3">
					<label class="flex items-center gap-2">
						<input type="checkbox" bind:checked={config.jsChallenge} class="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500" />
						<span class="text-sm text-gray-700 dark:text-gray-300">JavaScript Challenge</span>
					</label>
					<label class="flex items-center gap-2">
						<input type="checkbox" bind:checked={config.behaviorAnalysis} class="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500" />
						<span class="text-sm text-gray-700 dark:text-gray-300">Behavior Analysis</span>
					</label>
					<label class="flex items-center gap-2">
						<input type="checkbox" bind:checked={config.fingerprintCheck} class="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500" />
						<span class="text-sm text-gray-700 dark:text-gray-300">Browser Fingerprint</span>
					</label>
					<label class="flex items-center gap-2">
						<input type="checkbox" bind:checked={config.blockHeadless} class="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500" />
						<span class="text-sm text-gray-700 dark:text-gray-300">Block Headless Browsers</span>
					</label>
				</div>
			</div>

			<div class="border-t border-gray-200 dark:border-gray-700 pt-4">
				<h3 class="font-medium text-gray-800 dark:text-gray-200 mb-3">Network Filtering</h3>
				<div class="grid grid-cols-2 gap-3">
					<label class="flex items-center gap-2">
						<input type="checkbox" bind:checked={config.blockTor} class="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500" />
						<span class="text-sm text-gray-700 dark:text-gray-300">Block Tor Exit Nodes</span>
					</label>
					<label class="flex items-center gap-2">
						<input type="checkbox" bind:checked={config.blockVPN} class="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500" />
						<span class="text-sm text-gray-700 dark:text-gray-300">Block Known VPNs</span>
					</label>
				</div>
			</div>

			<div class="border-t border-gray-200 dark:border-gray-700 pt-4 grid grid-cols-2 gap-4">
				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Challenge Difficulty</label>
					<select bind:value={config.challengeDifficulty} class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm">
						<option value="low">Low (fast, less accurate)</option>
						<option value="medium">Medium (balanced)</option>
						<option value="high">High (slow, very accurate)</option>
					</select>
				</div>
				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Min Interaction Time (ms)</label>
					<input type="number" bind:value={config.minInteractionTime} class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
				</div>
				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Max Request Rate (per min)</label>
					<input type="number" bind:value={config.maxRequestRate} class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
				</div>
			</div>

			<div class="border-t border-gray-200 dark:border-gray-700 pt-4">
				<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Whitelisted IPs (one per line)</label>
				<textarea bind:value={config.whitelistedIPs} rows="3" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm font-mono"
					placeholder="192.168.1.0/24&#10;10.0.0.1"></textarea>
			</div>
		</div>

		<div class="flex gap-3 mt-6">
			<button on:click={saveConfig} disabled={saving} class="bg-cta-blue px-4 py-2 text-white rounded-md font-semibold hover:opacity-80 text-sm transition-all duration-200 disabled:opacity-50">
				{saving ? 'Saving...' : 'Save Configuration'}
			</button>
			<button on:click={cleanup} class="bg-gray-500 px-4 py-2 text-white rounded-md font-semibold hover:opacity-80 text-sm transition-all duration-200">
				Cleanup Expired Sessions
			</button>
		</div>
	</div>
{/if}
