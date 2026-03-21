<script>
	import { api } from '$lib/api/apiProxy.js';
	import Headline from '$lib/components/Headline.svelte';
	import HeadTitle from '$lib/components/HeadTitle.svelte';
	import { hideIsLoading, showIsLoading } from '$lib/store/loading';
	import { addToast } from '$lib/store/toast';
	import { onMount, onDestroy } from 'svelte';

	let isLoaded = false;
	let domains = [];
	let refreshInterval = null;

	// Default table params for fetching all domains
	const defaultOptions = { page: 1, rowsPerPage: 1000, sortBy: '', sortOrder: '' };

	// Domain rotation is managed via the backend DomainRotator service.
	// This page provides a read-only view of domain health status.
	// Domain CRUD is handled on the existing /domain/ page.

	async function fetchDomains() {
		try {
			const res = await api.domain.getAll(defaultOptions);
			if (res && res.data) {
				const items = Array.isArray(res.data) ? res.data : (res.data.items || []);
				domains = items.map((d) => ({
					...d,
					healthStatus: d.healthStatus || 'unknown',
					lastChecked: d.lastChecked || null,
					isActive: d.isActive !== false
				}));
			}
		} catch (e) {
			console.error('Failed to fetch domains:', e);
		}
	}

	function getStatusColor(status) {
		switch (status) {
			case 'healthy': return 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400';
			case 'flagged': return 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400';
			case 'warning': return 'bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-400';
			default: return 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-400';
		}
	}

	function getStatusDot(status) {
		switch (status) {
			case 'healthy': return 'bg-green-500';
			case 'flagged': return 'bg-red-500';
			case 'warning': return 'bg-amber-500';
			default: return 'bg-gray-400';
		}
	}

	onMount(async () => {
		showIsLoading();
		await fetchDomains();
		isLoaded = true;
		hideIsLoading();
		// refresh every 60 seconds
		refreshInterval = setInterval(fetchDomains, 60000);
	});

	onDestroy(() => {
		if (refreshInterval) clearInterval(refreshInterval);
	});
</script>

<HeadTitle title="Domain Rotation" />

<Headline title="Domain Rotation & Health" subtitle="Monitor domain reputation and manage automatic rotation. Domains are managed on the Domains page." />

<div class="mb-6 p-4 bg-blue-50 dark:bg-blue-900/20 rounded-lg border border-blue-200 dark:border-blue-800">
	<h3 class="text-sm font-semibold text-blue-800 dark:text-blue-300 mb-1">Automatic Domain Rotation</h3>
	<p class="text-sm text-blue-700 dark:text-blue-400">
		The backend DomainRotator service periodically checks each domain against Google Safe Browsing and DNS blacklists.
		If a domain is flagged, it is automatically deactivated and the next healthy standby domain takes over.
		Configure check intervals and thresholds in your backend configuration.
	</p>
</div>

{#if isLoaded}
	<div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
		<div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
			<h3 class="text-sm font-semibold text-gray-700 dark:text-gray-300">Domain Health Status</h3>
			<span class="text-xs text-gray-500 dark:text-gray-400">{domains.length} domains</span>
		</div>
		{#if domains.length === 0}
			<div class="p-8 text-center text-gray-500 dark:text-gray-400">
				No domains configured. Add domains on the <a href="/domain/" class="text-blue-600 dark:text-blue-400 underline">Domains page</a>.
			</div>
		{:else}
			<div class="overflow-x-auto">
				<table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
					<thead class="bg-gray-50 dark:bg-gray-900/50">
						<tr>
							<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Domain</th>
							<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Status</th>
							<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Health</th>
							<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Last Checked</th>
							<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Proxies</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-gray-200 dark:divide-gray-700">
						{#each domains as domain}
							<tr class="hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors">
								<td class="px-4 py-3 text-sm font-medium text-gray-900 dark:text-gray-100">
									{domain.name || domain.domain || 'N/A'}
								</td>
								<td class="px-4 py-3 text-sm">
									{#if domain.isActive}
										<span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400">
											Active
										</span>
									{:else}
										<span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400">
											Standby
										</span>
									{/if}
								</td>
								<td class="px-4 py-3 text-sm">
									<span class="inline-flex items-center gap-1.5">
										<span class="w-2 h-2 rounded-full {getStatusDot(domain.healthStatus)}"></span>
										<span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium {getStatusColor(domain.healthStatus)}">
											{domain.healthStatus}
										</span>
									</span>
								</td>
								<td class="px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
									{domain.lastChecked ? new Date(domain.lastChecked).toLocaleString() : 'Never'}
								</td>
								<td class="px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
									{domain.proxyCount || 0}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	</div>
{/if}
