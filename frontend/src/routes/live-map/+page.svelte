<script>
	import { api } from '$lib/api/apiProxy.js';
	import Headline from '$lib/components/Headline.svelte';
	import HeadTitle from '$lib/components/HeadTitle.svelte';
	import { hideIsLoading, showIsLoading } from '$lib/store/loading';
	import { onMount, onDestroy } from 'svelte';

	let mapContainer;
	let map = null;
	let markers = [];
	let events = [];
	let stats = null;
	let isLoaded = false;
	let refreshInterval = null;
	let selectedMinutes = 60;
	let leafletLoaded = false;
	let L = null;

	const minuteOptions = [
		{ value: 15, label: 'Last 15 min' },
		{ value: 30, label: 'Last 30 min' },
		{ value: 60, label: 'Last hour' },
		{ value: 360, label: 'Last 6 hours' },
		{ value: 1440, label: 'Last 24 hours' }
	];

	async function loadLeaflet() {
		if (leafletLoaded) return;
		// load Leaflet CSS
		const link = document.createElement('link');
		link.rel = 'stylesheet';
		link.href = 'https://unpkg.com/leaflet@1.9.4/dist/leaflet.css';
		document.head.appendChild(link);
		// load Leaflet JS
		const script = document.createElement('script');
		script.src = 'https://unpkg.com/leaflet@1.9.4/dist/leaflet.js';
		await new Promise((resolve) => {
			script.onload = resolve;
			document.head.appendChild(script);
		});
		L = window.L;
		leafletLoaded = true;
	}

	function initMap() {
		if (!L || !mapContainer) return;
		map = L.map(mapContainer, {
			center: [20, 0],
			zoom: 2,
			minZoom: 2,
			maxZoom: 18,
			zoomControl: true
		});
		L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', {
			attribution: '&copy; OpenStreetMap contributors &copy; CARTO',
			subdomains: 'abcd',
			maxZoom: 20
		}).addTo(map);
	}

	function getEventColor(eventType) {
		switch (eventType) {
			case 'submit': return '#ef4444';
			case 'proxy_submit': return '#ef4444';
			case 'cookie_bundle': return '#f59e0b';
			case 'proxy_cookie': return '#f59e0b';
			case 'proxy_visit': return '#8b5cf6';
			default: return '#3b82f6';
		}
	}

	function getEventLabel(eventType) {
		switch (eventType) {
			case 'submit': return 'Submit';
			case 'proxy_submit': return 'Proxy Submit';
			case 'cookie_bundle': return 'Cookie';
			case 'proxy_cookie': return 'Proxy Cookie';
			case 'proxy_visit': return 'Proxy Visit';
			case 'visit': return 'Visit';
			default: return eventType || 'Visit';
		}
	}

	function updateMarkers() {
		if (!map || !L) return;
		// clear existing markers
		markers.forEach((m) => map.removeLayer(m));
		markers = [];

		events.forEach((event) => {
			if (!event.latitude || !event.longitude) return;
			const color = getEventColor(event.eventType);
			const icon = L.divIcon({
				className: 'custom-marker',
				html: `<div style="
					width: 12px; height: 12px;
					background: ${color};
					border-radius: 50%;
					border: 2px solid white;
					box-shadow: 0 0 8px ${color}80;
					animation: pulse 2s infinite;
				"></div>`,
				iconSize: [12, 12],
				iconAnchor: [6, 6]
			});
			const marker = L.marker([event.latitude, event.longitude], { icon }).addTo(map);
			// Backend sends: timestamp, ipAddress, campaignId, eventType, city, country
			const time = new Date(event.timestamp).toLocaleString();
			marker.bindPopup(`
				<div style="font-size: 12px; min-width: 180px;">
					<strong>${getEventLabel(event.eventType)}</strong><br/>
					<span style="color: #666;">IP:</span> ${event.ipAddress || 'N/A'}<br/>
					<span style="color: #666;">Country:</span> ${event.country || 'N/A'}<br/>
					<span style="color: #666;">City:</span> ${event.city || 'N/A'}<br/>
					<span style="color: #666;">Time:</span> ${time}<br/>
					<span style="color: #666;">${event.eventType && event.eventType.startsWith('proxy_') ? 'Domain' : 'Campaign'}:</span> ${event.campaignId || 'N/A'}
				</div>
			`);
			markers.push(marker);
		});
	}

	async function fetchData() {
		try {
			const [eventsRes, statsRes] = await Promise.all([
				api.liveMap.getRecentEvents(selectedMinutes, 500),
				api.liveMap.getGeoStats(7)
			]);
			if (eventsRes && eventsRes.data) {
				events = Array.isArray(eventsRes.data) ? eventsRes.data : (eventsRes.data.items || []);
				updateMarkers();
			}
			if (statsRes && statsRes.data) {
				stats = statsRes.data;
			}
		} catch (e) {
			console.error('Failed to fetch live map data:', e);
		}
	}

	async function onTimeRangeChange() {
		await fetchData();
	}

	onMount(async () => {
		showIsLoading();
		await loadLeaflet();
		initMap();
		await fetchData();
		isLoaded = true;
		hideIsLoading();
		// auto-refresh every 30 seconds
		refreshInterval = setInterval(fetchData, 30000);
	});

	onDestroy(() => {
		if (refreshInterval) clearInterval(refreshInterval);
		if (map) map.remove();
	});
</script>

<svelte:head>
	<style>
		@keyframes pulse {
			0% { opacity: 1; transform: scale(1); }
			50% { opacity: 0.7; transform: scale(1.3); }
			100% { opacity: 1; transform: scale(1); }
		}
	</style>
</svelte:head>

<HeadTitle title="Live Map" />

<Headline title="Live Map" subtitle="Real-time geographic visualization of campaign events." />

<div class="mb-4 flex items-center justify-between">
	<div class="flex items-center gap-3">
		<label class="text-sm font-medium text-gray-700 dark:text-gray-300">Time Range:</label>
		<select
			bind:value={selectedMinutes}
			on:change={onTimeRangeChange}
			class="rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm"
		>
			{#each minuteOptions as opt}
				<option value={opt.value}>{opt.label}</option>
			{/each}
		</select>
	</div>
	<div class="flex items-center gap-4 text-sm">
		<span class="flex items-center gap-1">
			<span class="inline-block w-3 h-3 rounded-full bg-blue-500"></span> Visit
		</span>
		<span class="flex items-center gap-1">
			<span class="inline-block w-3 h-3 rounded-full bg-red-500"></span> Submit
		</span>
		<span class="flex items-center gap-1">
			<span class="inline-block w-3 h-3 rounded-full bg-amber-500"></span> Cookie
		</span>
		<span class="flex items-center gap-1">
			<span class="inline-block w-3 h-3 rounded-full" style="background: #8b5cf6;"></span> Proxy Visit
		</span>
		<span class="text-gray-500 dark:text-gray-400">{events.length} events</span>
	</div>
</div>

<div class="rounded-lg overflow-hidden border border-gray-200 dark:border-gray-700 shadow-sm" style="height: 500px;">
	<div bind:this={mapContainer} style="height: 100%; width: 100%;"></div>
</div>

{#if stats}
	<div class="mt-6 grid grid-cols-1 md:grid-cols-3 gap-4">
		<div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
			<h3 class="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">Summary</h3>
			<div class="space-y-2">
				<div class="flex items-center justify-between text-sm">
					<span class="text-gray-600 dark:text-gray-400">Total Events</span>
					<span class="font-medium text-gray-900 dark:text-gray-100">{stats.totalEvents || 0}</span>
				</div>
				<div class="flex items-center justify-between text-sm">
					<span class="text-gray-600 dark:text-gray-400">Active Countries</span>
					<span class="font-medium text-gray-900 dark:text-gray-100">{stats.activeCountries || 0}</span>
				</div>
				<div class="flex items-center justify-between text-sm">
					<span class="text-gray-600 dark:text-gray-400">Active Cities</span>
					<span class="font-medium text-gray-900 dark:text-gray-100">{stats.activeCities || 0}</span>
				</div>
			</div>
		</div>
		{#if stats.eventsByCountry && Object.keys(stats.eventsByCountry).length > 0}
			<div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
				<h3 class="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">Top Countries</h3>
				<div class="space-y-2">
					{#each Object.entries(stats.eventsByCountry).sort((a, b) => b[1] - a[1]).slice(0, 10) as [country, count]}
						<div class="flex items-center justify-between text-sm">
							<span class="text-gray-600 dark:text-gray-400">{country || 'Unknown'}</span>
							<span class="font-medium text-gray-900 dark:text-gray-100">{count}</span>
						</div>
					{/each}
				</div>
			</div>
		{/if}
		{#if stats.eventsByType && Object.keys(stats.eventsByType).length > 0}
			<div class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
				<h3 class="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">Events by Type</h3>
				<div class="space-y-2">
					{#each Object.entries(stats.eventsByType) as [type, count]}
						<div class="flex items-center justify-between text-sm">
							<span class="text-gray-600 dark:text-gray-400">{type}</span>
							<span class="font-medium text-gray-900 dark:text-gray-100">{count}</span>
						</div>
					{/each}
				</div>
			</div>
		{/if}
	</div>
{/if}
