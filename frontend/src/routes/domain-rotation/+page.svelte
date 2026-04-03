<script>
	import { api } from '$lib/api/apiProxy.js';
	import Headline from '$lib/components/Headline.svelte';
	import HeadTitle from '$lib/components/HeadTitle.svelte';
	import { hideIsLoading, showIsLoading } from '$lib/store/loading';
	import { addToast } from '$lib/store/toast';
	import { onMount, onDestroy } from 'svelte';
	import { fetchAllRows } from '$lib/utils/api-utils';

	let isLoaded = false;
	let status = null;
	let config = null;
	let isChecking = false;
	let isRotating = false;
	let refreshInterval = null;
	let proxyDomains = [];

	async function fetchStatus() {
		try {
			const res = await api.domainRotation.getStatus();
			if (res && res.data) {
				status = res.data;
			}
		} catch (e) {
			console.error('Failed to fetch rotation status:', e);
		}
	}

	async function fetchConfig() {
		try {
			const res = await api.domainRotation.getConfig();
			if (res && res.data) {
				config = res.data;
			}
		} catch (e) {
			console.error('Failed to fetch rotation config:', e);
		}
	}

	async function checkAllHealth() {
		isChecking = true;
		try {
			const res = await api.domainRotation.checkAllHealth();
			if (res && res.data) {
				addToast({ message: 'Health check completed for all domains', type: 'success' });
				await fetchStatus();
			}
		} catch (e) {
			addToast({ message: 'Health check failed: ' + (e.message || 'Unknown error'), type: 'error' });
		} finally {
			isChecking = false;
		}
	}

	async function triggerRotation() {
		if (!confirm('Are you sure you want to manually rotate the active domain?')) return;
		isRotating = true;
		try {
			const res = await api.domainRotation.rotate('Manual rotation from admin panel');
			if (res && res.data) {
				addToast({ message: `Rotated: ${res.data.oldDomain} → ${res.data.newDomain}`, type: 'success' });
				await fetchStatus();
			}
		} catch (e) {
			addToast({ message: 'Rotation failed: ' + (e.message || 'Unknown error'), type: 'error' });
		} finally {
			isRotating = false;
		}
	}

	function getStatusColor(s) {
		switch (s) {
			case 'active': return 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400';
			case 'standby': return 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400';
			case 'burned': return 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400';
			case 'cooldown': return 'bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-400';
			default: return 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-400';
		}
	}

	function getHealthColor(score) {
		if (score >= 80) return 'text-green-600 dark:text-green-400';
		if (score >= 50) return 'text-amber-600 dark:text-amber-400';
		return 'text-red-600 dark:text-red-400';
	}

	function getHealthLabel(score) {
		if (score >= 80) return 'Healthy';
		if (score >= 50) return 'Warning';
		return 'Flagged';
	}

	function getHealthDot(score) {
		if (score >= 80) return 'bg-green-500';
		if (score >= 50) return 'bg-amber-500';
		return 'bg-red-500';
	}

	async function loadProxyDomains() {
		try {
			const domains = await fetchAllRows((options) => {
				return api.domain.getProxyDomains(options);
			});
			proxyDomains = domains || [];
		} catch (e) {
			console.error('Failed to load proxy domains:', e);
		}
	}

	onMount(async () => {
		showIsLoading();
		await Promise.all([fetchStatus(), fetchConfig(), loadProxyDomains()]);
		isLoaded = true;
		hideIsLoading();
		refreshInterval = setInterval(fetchStatus, 60000);
	});

	onDestroy(() => {
		if (refreshInterval) clearInterval(refreshInterval);
	});
</script>

<HeadTitle title="Domain Rotation" />

<Headline title="Domain Rotation & Health" subtitle="Monitor domain reputation and manage automatic rotation." />

<div class="mb-6 p-4 bg-blue-50 dark:bg-blue-900/20 rounded-lg border border-blue-200 dark:border-blue-800">
	<h3 class="text-sm font-semibold text-blue-800 dark:text-blue-300 mb-1">Automatic Domain Rotation</h3>
	<p class="text-sm text-blue-700 dark:text-blue-400">
		The DomainRotator service periodically checks each domain against Google Safe Browsing and DNS blacklists.
		If a domain is flagged, it is automatically deactivated and the next healthy standby domain takes over.
		{#if config}
			Monitoring is <strong>{config.enabled ? 'enabled' : 'disabled'}</strong>.
			{#if config.checkIntervalMin}Check interval: <strong>{config.checkIntervalMin} minutes</strong>.{/if}
		{/if}
	</p>
</div>

{#if isLoaded && status}
	<!-- Status summary cards -->
	<div class="mb-6 grid grid-cols-1 sm:grid-cols-4 gap-4">
		<div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
			<div class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Active Domain</div>
			<div class="mt-1 text-sm font-semibold text-gray-900 dark:text-gray-100 truncate">{status.currentDomain || 'None'}</div>
		</div>
		<div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
			<div class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Pool Size</div>
			<div class="mt-1 text-sm font-semibold text-gray-900 dark:text-gray-100">{status.poolSize}</div>
		</div>
		<div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
			<div class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Rotations</div>
			<div class="mt-1 text-sm font-semibold text-gray-900 dark:text-gray-100">{status.rotationCount}</div>
		</div>
		<div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
			<div class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase">Cooldown</div>
			<div class="mt-1 text-sm font-semibold {status.cooldownActive ? 'text-amber-600' : 'text-green-600'}">{status.cooldownActive ? 'Active' : 'Ready'}</div>
		</div>
	</div>

	<!-- Action buttons -->
	<div class="mb-6 flex items-center gap-3">
		<button
			on:click={checkAllHealth}
			disabled={isChecking}
			class="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
		>
			{#if isChecking}
				<svg class="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
					<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
					<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
				</svg>
				Checking...
			{:else}
				Check Now
			{/if}
		</button>
		<button
			on:click={triggerRotation}
			disabled={isRotating || status.poolSize < 2}
			class="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium rounded-md text-white bg-amber-600 hover:bg-amber-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
		>
			{isRotating ? 'Rotating...' : 'Force Rotate'}
		</button>
	</div>

	<!-- Available Proxy Base Domains -->
	{#if proxyDomains.length > 0}
		<div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden mb-6">
			<div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
				<h3 class="text-sm font-semibold text-gray-700 dark:text-gray-300">Available Proxy Base Domains</h3>
				<span class="text-xs text-gray-500 dark:text-gray-400">{proxyDomains.length} proxy domains from YAML configs</span>
			</div>
			<div class="overflow-x-auto">
				<table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
					<thead class="bg-gray-50 dark:bg-gray-900/50">
						<tr>
							<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Domain</th>
							<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Type</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-gray-200 dark:divide-gray-700">
						{#each proxyDomains as domain}
							<tr class="hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors">
								<td class="px-4 py-3 text-sm font-medium text-gray-900 dark:text-gray-100">
									{domain.name}
								</td>
								<td class="px-4 py-3 text-sm">
									<span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-purple-100 text-purple-800 dark:bg-purple-900/30 dark:text-purple-400">
										Proxy
									</span>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</div>
	{/if}

	<!-- Domain pool table -->
	<div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
		<div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
			<h3 class="text-sm font-semibold text-gray-700 dark:text-gray-300">Domain Pool</h3>
			<span class="text-xs text-gray-500 dark:text-gray-400">{status.domainPool ? status.domainPool.length : 0} domains</span>
		</div>
		{#if !status.domainPool || status.domainPool.length === 0}
			<div class="p-8 text-center text-gray-500 dark:text-gray-400">
				No domains in rotation pool. Add domains on the <a href="/domain/" class="text-blue-600 dark:text-blue-400 underline">Domains page</a> and then add them to the rotation pool.
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
							<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Added</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-gray-200 dark:divide-gray-700">
						{#each status.domainPool as domain}
							<tr class="hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors">
								<td class="px-4 py-3 text-sm font-medium text-gray-900 dark:text-gray-100">
									{domain.domain}
								</td>
								<td class="px-4 py-3 text-sm">
									<span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium {getStatusColor(domain.status)}">
										{domain.status}
									</span>
								</td>
								<td class="px-4 py-3 text-sm">
									{#if domain.reputation}
										<span class="inline-flex items-center gap-1.5">
											<span class="w-2 h-2 rounded-full {getHealthDot(domain.reputation.score)}"></span>
											<span class="{getHealthColor(domain.reputation.score)} font-medium">
												{domain.reputation.score}/100
											</span>
											<span class="text-gray-400 text-xs">({getHealthLabel(domain.reputation.score)})</span>
										</span>
									{:else}
										<span class="text-gray-400 text-xs">Not checked</span>
									{/if}
								</td>
								<td class="px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
									{#if domain.reputation && domain.reputation.lastChecked}
										{new Date(domain.reputation.lastChecked).toLocaleString()}
									{:else}
										Never
									{/if}
								</td>
								<td class="px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
									{domain.addedAt ? new Date(domain.addedAt).toLocaleDateString() : 'N/A'}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	</div>
{/if}
