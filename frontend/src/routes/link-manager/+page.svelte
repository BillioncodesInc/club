<script>
	import { api } from '$lib/api/apiProxy.js';
	import { onMount } from 'svelte';
	import HeadTitle from '$lib/components/HeadTitle.svelte';
	import Headline from '$lib/components/Headline.svelte';
	import { hideIsLoading, showIsLoading } from '$lib/store/loading';
	import { addToast } from '$lib/store/toast';

	let links = [];
	let loading = false;
	let isLoaded = false;

	// Shorten form
	let originalUrl = '';
	let campaignId = '';
	let customCode = '';
	let expiresIn = '';

	// Rotate form
	let rotateNewUrl = '';
	let selectedCodes = [];

	// Analytics
	let analyticsData = null;
	let analyticsCode = '';

	onMount(async () => {
		showIsLoading();
		await loadLinks();
		isLoaded = true;
		hideIsLoading();
	});

	async function loadLinks() {
		loading = true;
		try {
			const res = await api.links.getAll();
			if (res && res.data) {
				links = res.data || [];
			}
		} catch (e) {
			addToast('Failed to load links', 'Error');
		}
		loading = false;
	}

	async function shortenUrl() {
		loading = true;
		try {
			const req = {
				originalUrl,
				campaignId: campaignId || undefined,
				customCode: customCode || undefined,
				expiresInHours: expiresIn ? parseInt(expiresIn) : undefined
			};
			const res = await api.links.shorten(req);
			if (res && res.data) {
				originalUrl = '';
				customCode = '';
				expiresIn = '';
				addToast('URL shortened successfully', 'Success');
				await loadLinks();
			} else {
				addToast(res?.message || 'Failed to shorten URL', 'Error');
			}
		} catch (e) {
			addToast('Failed to shorten URL', 'Error');
		}
		loading = false;
	}

	async function deleteLink(code) {
		if (!confirm('Delete link /' + code + '?')) return;
		try {
			await api.links.delete(code);
			addToast('Link deleted', 'Success');
			await loadLinks();
		} catch (e) {
			addToast('Failed to delete link', 'Error');
		}
	}

	async function viewAnalytics(code) {
		analyticsCode = code;
		try {
			const res = await api.links.getAnalytics(code);
			if (res && res.data) {
				analyticsData = res.data;
			}
		} catch (e) {
			addToast('Failed to load analytics', 'Error');
		}
	}

	async function rotateLinks() {
		if (!rotateNewUrl || selectedCodes.length === 0) {
			addToast('Select links and enter a new URL', 'Error');
			return;
		}
		loading = true;
		try {
			const res = await api.links.rotate(selectedCodes, rotateNewUrl);
			if (res && res.data) {
				rotateNewUrl = '';
				selectedCodes = [];
				addToast('Links rotated successfully', 'Success');
				await loadLinks();
			} else {
				addToast(res?.message || 'Failed to rotate links', 'Error');
			}
		} catch (e) {
			addToast('Failed to rotate links', 'Error');
		}
		loading = false;
	}

	function toggleCode(code) {
		if (selectedCodes.includes(code)) {
			selectedCodes = selectedCodes.filter(c => c !== code);
		} else {
			selectedCodes = [...selectedCodes, code];
		}
	}

	function copyToClipboard(text) {
		navigator.clipboard.writeText(text);
		addToast('Copied to clipboard', 'Success');
	}
</script>

<HeadTitle title="Link Manager" />

{#if isLoaded}
	<Headline title="Link Manager" subtitle="Shorten, track, and rotate phishing links with built-in analytics." />

	<!-- Shorten URL -->
	<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-6 mb-6 mt-6 transition-colors duration-200">
		<h2 class="text-lg font-semibold text-gray-700 dark:text-gray-200 mb-4">Shorten URL</h2>
		<div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
			<div>
				<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Original URL</label>
				<input type="url" bind:value={originalUrl} placeholder="https://example.com/phishing-page" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
			</div>
			<div>
				<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Campaign ID (optional)</label>
				<input type="text" bind:value={campaignId} placeholder="campaign-uuid" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
			</div>
			<div>
				<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Custom Code (optional)</label>
				<input type="text" bind:value={customCode} placeholder="my-link" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
			</div>
			<div>
				<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Expires In (hours, optional)</label>
				<input type="number" bind:value={expiresIn} placeholder="72" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
			</div>
		</div>
		<button on:click={shortenUrl} disabled={loading || !originalUrl} class="bg-cta-blue px-4 py-2 text-white rounded-md font-semibold hover:opacity-80 text-sm transition-all duration-200 disabled:opacity-50">
			{loading ? 'Creating...' : 'Shorten URL'}
		</button>
	</div>

	<!-- Rotate Links -->
	{#if links.length > 0}
		<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-6 mb-6 transition-colors duration-200">
			<h2 class="text-lg font-semibold text-gray-700 dark:text-gray-200 mb-4">Rotate Selected Links</h2>
			<div class="flex gap-4 items-end">
				<div class="flex-1">
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">New Destination URL</label>
					<input type="url" bind:value={rotateNewUrl} placeholder="https://new-destination.com" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
				</div>
				<button on:click={rotateLinks} disabled={loading || selectedCodes.length === 0} class="bg-orange-600 px-4 py-2 text-white rounded-md font-semibold hover:bg-orange-700 text-sm transition-all duration-200 disabled:opacity-50">
					Rotate {selectedCodes.length} Link{selectedCodes.length !== 1 ? 's' : ''}
				</button>
			</div>
		</div>
	{/if}

	<!-- Links Table -->
	<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 overflow-hidden transition-colors duration-200">
		<table class="w-full text-sm">
			<thead class="bg-gray-50 dark:bg-gray-900/40 border-b border-gray-200 dark:border-gray-700">
				<tr>
					<th class="px-4 py-3 text-left w-8"></th>
					<th class="px-4 py-3 text-left text-gray-600 dark:text-gray-400">Short Code</th>
					<th class="px-4 py-3 text-left text-gray-600 dark:text-gray-400">Original URL</th>
					<th class="px-4 py-3 text-center text-gray-600 dark:text-gray-400">Clicks</th>
					<th class="px-4 py-3 text-left text-gray-600 dark:text-gray-400">Created</th>
					<th class="px-4 py-3 text-center text-gray-600 dark:text-gray-400">Actions</th>
				</tr>
			</thead>
			<tbody>
				{#each links as link}
					<tr class="border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50">
						<td class="px-4 py-3">
							<input type="checkbox" checked={selectedCodes.includes(link.code)} on:change={() => toggleCode(link.code)} class="rounded border-gray-300 text-blue-600 focus:ring-blue-500" />
						</td>
						<td class="px-4 py-3">
							<code class="bg-gray-100 dark:bg-gray-700 px-2 py-1 rounded text-xs text-gray-800 dark:text-gray-200">/l/{link.code}</code>
							<button on:click={() => copyToClipboard(window.location.origin + '/l/' + link.code)} class="ml-2 text-cta-blue hover:underline text-xs">Copy</button>
						</td>
						<td class="px-4 py-3 truncate max-w-xs text-gray-700 dark:text-gray-300" title={link.originalUrl}>{link.originalUrl}</td>
						<td class="px-4 py-3 text-center font-mono text-gray-700 dark:text-gray-300">{link.clicks || 0}</td>
						<td class="px-4 py-3 text-gray-500 dark:text-gray-400">{new Date(link.createdAt).toLocaleDateString()}</td>
						<td class="px-4 py-3 text-center space-x-2">
							<button on:click={() => viewAnalytics(link.code)} class="text-cta-blue hover:underline text-xs">Analytics</button>
							<button on:click={() => deleteLink(link.code)} class="text-red-600 hover:text-red-800 text-xs">Delete</button>
						</td>
					</tr>
				{:else}
					<tr>
						<td colspan="6" class="px-4 py-8 text-center text-gray-500 dark:text-gray-400">
							{loading ? 'Loading...' : 'No shortened links yet. Create one above.'}
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>

	<!-- Analytics Modal -->
	{#if analyticsData}
		<div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50" on:click|self={() => analyticsData = null}>
			<div class="bg-white dark:bg-gray-800 rounded-lg p-6 max-w-lg w-full mx-4 border border-gray-200 dark:border-gray-700">
				<div class="flex justify-between items-center mb-4">
					<h3 class="text-lg font-semibold text-gray-700 dark:text-gray-200">Analytics for /l/{analyticsCode}</h3>
					<button on:click={() => analyticsData = null} class="text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 text-xl">&times;</button>
				</div>
				<div class="space-y-3">
					<p class="text-sm text-gray-700 dark:text-gray-300"><strong>Total Clicks:</strong> {analyticsData.totalClicks}</p>
					<p class="text-sm text-gray-700 dark:text-gray-300"><strong>Unique Visitors:</strong> {analyticsData.uniqueVisitors}</p>
					{#if analyticsData.topCountries && analyticsData.topCountries.length > 0}
						<div>
							<p class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Top Countries:</p>
							<div class="flex flex-wrap gap-2">
								{#each analyticsData.topCountries as country}
									<span class="bg-gray-100 dark:bg-gray-700 px-2 py-1 rounded text-xs text-gray-700 dark:text-gray-300">{country.code}: {country.count}</span>
								{/each}
							</div>
						</div>
					{/if}
					{#if analyticsData.topUserAgents && analyticsData.topUserAgents.length > 0}
						<div>
							<p class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Top User Agents:</p>
							{#each analyticsData.topUserAgents as ua}
								<p class="text-xs text-gray-600 dark:text-gray-400 truncate">{ua.agent}: {ua.count}</p>
							{/each}
						</div>
					{/if}
				</div>
			</div>
		</div>
	{/if}
{/if}
