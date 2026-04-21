<script>
	import { api } from '$lib/api/apiProxy.js';
	import Headline from '$lib/components/Headline.svelte';
	import HeadTitle from '$lib/components/HeadTitle.svelte';
	import { hideIsLoading, showIsLoading } from '$lib/store/loading';
	import { onMount, onDestroy } from 'svelte';
	import 'leaflet/dist/leaflet.css';
	import 'leaflet.markercluster/dist/MarkerCluster.css';
	import 'leaflet.markercluster/dist/MarkerCluster.Default.css';

	let mapContainer;
	let map = null;
	let markerClusterGroup = null;
	let heatLayer = null;
	let events = [];
	let stats = null;
	let isLoaded = false;
	let refreshInterval = null;
	let selectedMinutes = 60;
	let leafletLoaded = false;
	let L = null;
	let showBots = false;
	let showHeatmap = true;
	let animationQueue = [];
	let isAnimating = false;
	let previousEventIds = new Set();

	// --- Improvement #4 (time-range tabs) + #5 (pause/resume) + #12 (tab-visibility) + #3 (event-type toggles) ---
	// Quick-access tabs rendered alongside the existing dropdown.
	const quickRanges = [
		{ value: 15, label: '15m' },
		{ value: 60, label: '1h' },
		{ value: 360, label: '6h' },
		{ value: 1440, label: '24h' }
	];
	let isPaused = false; // operator-controlled pause of the refresh loop
	let isTabHidden = false; // auto-pause when document.visibilityState === 'hidden'
	// Per-event-type visibility toggles. Treating proxy_submit as a Submit
	// and proxy_cookie as a Cookie for the purposes of the legend pills.
	let typeVisibility = {
		visit: true,
		submit: true,
		cookie: true,
		proxy_visit: true
	};
	// Family membership: map a raw event type into one of the four legend buckets.
	function eventFamily(t) {
		switch (t) {
			case 'submit':
			case 'proxy_submit':
				return 'submit';
			case 'cookie_bundle':
			case 'proxy_cookie':
				return 'cookie';
			case 'proxy_visit':
				return 'proxy_visit';
			default:
				return 'visit';
		}
	}

	const minuteOptions = [
		{ value: 15, label: 'Last 15 min' },
		{ value: 30, label: 'Last 30 min' },
		{ value: 60, label: 'Last hour' },
		{ value: 360, label: 'Last 6 hours' },
		{ value: 1440, label: 'Last 24 hours' }
	];

	async function loadLeaflet() {
		if (leafletLoaded) return;
		// Dynamic imports so Leaflet (which touches `window`) only loads in the browser.
		const leafletModule = await import('leaflet');
		L = leafletModule.default || leafletModule;
		// markercluster and heat attach to the global L instance when loaded
		window.L = L;
		await import('leaflet.markercluster');
		await import('leaflet.heat');
		leafletLoaded = true;
	}

	function initMap() {
		if (!L || !mapContainer) return;
		map = L.map(mapContainer, {
			center: [20, 0],
			zoom: 2,
			minZoom: 2,
			maxZoom: 18,
			zoomControl: true,
			preferCanvas: true
		});

		// Dark theme tile layer
		L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', {
			attribution: '&copy; OpenStreetMap contributors &copy; CARTO',
			subdomains: 'abcd',
			maxZoom: 20
		}).addTo(map);

		// Initialize marker cluster group
		markerClusterGroup = L.markerClusterGroup({
			maxClusterRadius: 50,
			spiderfyOnMaxZoom: true,
			showCoverageOnHover: true,
			zoomToBoundsOnClick: true,
			disableClusteringAtZoom: 15,
			animateAddingMarkers: true,
			iconCreateFunction: function (cluster) {
				const childCount = cluster.getChildCount();
				let size = 'small';
				let dim = 30;
				if (childCount >= 100) {
					size = 'large';
					dim = 50;
				} else if (childCount >= 10) {
					size = 'medium';
					dim = 40;
				}
				// Determine dominant event type color
				const children = cluster.getAllChildMarkers();
				let colorCounts = {};
				let hasBots = false;
				children.forEach((m) => {
					const color = m.options._eventColor || '#3b82f6';
					colorCounts[color] = (colorCounts[color] || 0) + 1;
					if (m.options._isBot) hasBots = true;
				});
				let dominantColor = '#3b82f6';
				let maxCount = 0;
				for (const [color, count] of Object.entries(colorCounts)) {
					if (count > maxCount) {
						maxCount = count;
						dominantColor = color;
					}
				}
				const borderColor = hasBots ? 'rgba(239,68,68,0.6)' : 'rgba(255,255,255,0.8)';
				return L.divIcon({
					html: `<div style="
						background: ${dominantColor};
						color: white;
						border-radius: 50%;
						width: ${dim}px;
						height: ${dim}px;
						display: flex;
						align-items: center;
						justify-content: center;
						font-size: ${dim > 40 ? 14 : 12}px;
						font-weight: bold;
						border: 3px solid ${borderColor};
						box-shadow: 0 0 15px ${dominantColor}80, 0 0 30px ${dominantColor}40;
						transition: all 0.3s ease;
					">${childCount}</div>`,
					className: 'custom-cluster-icon',
					iconSize: L.point(dim, dim)
				});
			}
		});
		map.addLayer(markerClusterGroup);
	}

	function getEventColor(eventType, isBot) {
		if (isBot) return '#6b7280'; // gray for bots
		switch (eventType) {
			case 'submit':
			case 'proxy_submit':
				return '#ef4444'; // red
			case 'cookie_bundle':
			case 'proxy_cookie':
				return '#f59e0b'; // amber
			case 'proxy_visit':
				return '#8b5cf6'; // purple
			default:
				return '#3b82f6'; // blue
		}
	}

	function getEventLabel(eventType) {
		switch (eventType) {
			case 'submit':
				return 'Submit';
			case 'proxy_submit':
				return 'Proxy Submit';
			case 'cookie_bundle':
				return 'Cookie';
			case 'proxy_cookie':
				return 'Proxy Cookie';
			case 'proxy_visit':
				return 'Proxy Visit';
			case 'visit':
				return 'Visit';
			default:
				return eventType || 'Visit';
		}
	}

	function getEventWeight(eventType) {
		switch (eventType) {
			case 'submit':
			case 'proxy_submit':
				return 1.0;
			case 'cookie_bundle':
			case 'proxy_cookie':
				return 0.8;
			case 'proxy_visit':
				return 0.4;
			default:
				return 0.3;
		}
	}

	// Create a ripple animation at a given lat/lng
	function createRipple(lat, lng, color) {
		if (!map || !L) return;
		const ripple = L.circleMarker([lat, lng], {
			radius: 5,
			color: color,
			fillColor: color,
			fillOpacity: 0.6,
			weight: 2,
			opacity: 0.8,
			className: 'ripple-marker'
		}).addTo(map);

		let radius = 5;
		let opacity = 0.8;
		const animate = () => {
			radius += 1.5;
			opacity -= 0.03;
			if (opacity <= 0) {
				map.removeLayer(ripple);
				return;
			}
			ripple.setRadius(radius);
			ripple.setStyle({ opacity: opacity, fillOpacity: opacity * 0.5 });
			requestAnimationFrame(animate);
		};
		requestAnimationFrame(animate);
	}

	// Process animation queue for new events
	function processAnimationQueue() {
		if (isAnimating || animationQueue.length === 0) return;
		isAnimating = true;

		const event = animationQueue.shift();
		if (event && event.latitude && event.longitude) {
			const color = getEventColor(event.eventType, event.isBot);
			createRipple(event.latitude, event.longitude, color);
		}

		setTimeout(() => {
			isAnimating = false;
			processAnimationQueue();
		}, 200);
	}

	// --- Improvement #3: apply event-type toggle filter ---
	function applyTypeFilter(list) {
		return list.filter((e) => typeVisibility[eventFamily(e.eventType)] !== false);
	}

	// --- Improvement #1: hover-tooltip helper ---
	// Returns a concise tooltip (event label + country + relative time) vs
	// the full popup shown on click. Used by marker.bindTooltip(...).
	function buildTooltipHTML(event) {
		return (
			'<div style="font-size:11px;line-height:1.4">' +
			'<strong>' +
			getEventLabel(event.eventType) +
			'</strong>' +
			(event.isBot ? ' <span style="color:#6b7280">(bot)</span>' : '') +
			'<br/>' +
			(event.country || 'Unknown') +
			(event.city ? ', ' + event.city : '') +
			'<br/><span style="color:#9ca3af">' +
			relativeTime(event.timestamp) +
			'</span></div>'
		);
	}

	// Format a timestamp as a short "x s/m/h ago" string.
	function relativeTime(ts) {
		if (!ts) return 'just now';
		const diffMs = Date.now() - new Date(ts).getTime();
		const s = Math.max(0, Math.floor(diffMs / 1000));
		if (s < 60) return s + 's ago';
		const m = Math.floor(s / 60);
		if (m < 60) return m + 'm ago';
		const h = Math.floor(m / 60);
		if (h < 24) return h + 'h ago';
		return Math.floor(h / 24) + 'd ago';
	}

	// --- Improvement #9: unicode country-flag emoji from ISO-3166 alpha-2. ---
	// Maps 'US' -> regional-indicator pair, rendered as a flag on most platforms.
	function countryFlag(code) {
		if (!code || code.length !== 2) return '';
		const cc = code.toUpperCase();
		const OFFSET = 127397; // regional indicator A (U+1F1E6) - 'A'.charCodeAt(0)
		return String.fromCodePoint(cc.charCodeAt(0) + OFFSET, cc.charCodeAt(1) + OFFSET);
	}

	function updateHeatmap() {
		if (!map || !L || !window.L.heatLayer) return;

		// Remove existing heatmap
		if (heatLayer) {
			map.removeLayer(heatLayer);
			heatLayer = null;
		}

		if (!showHeatmap) return;

		const filteredEvents = applyTypeFilter(showBots ? events : events.filter((e) => !e.isBot));
		const heatData = filteredEvents
			.filter((e) => e.latitude && e.longitude)
			.map((e) => [e.latitude, e.longitude, getEventWeight(e.eventType)]);

		if (heatData.length > 0) {
			heatLayer = L.heatLayer(heatData, {
				radius: 25,
				blur: 20,
				maxZoom: 10,
				max: 1.0,
				gradient: {
					0.2: '#1e40af',
					0.4: '#3b82f6',
					0.6: '#8b5cf6',
					0.8: '#f59e0b',
					1.0: '#ef4444'
				}
			}).addTo(map);
		}
	}

	function updateMarkers() {
		if (!map || !L || !markerClusterGroup) return;
		markerClusterGroup.clearLayers();

		const filteredEvents = applyTypeFilter(showBots ? events : events.filter((e) => !e.isBot));

		// Detect new events for animation
		const currentIds = new Set(filteredEvents.map((e) => e.id));
		const newEvents = filteredEvents.filter((e) => !previousEventIds.has(e.id));
		previousEventIds = currentIds;

		// Queue new events for ripple animation
		newEvents.forEach((event) => {
			animationQueue.push(event);
		});
		processAnimationQueue();

		filteredEvents.forEach((event) => {
			if (!event.latitude || !event.longitude) return;
			const color = getEventColor(event.eventType, event.isBot);
			const isNew = newEvents.some((e) => e.id === event.id);
			const pulseClass = isNew ? 'pulse-new' : 'pulse-steady';
			const size = event.isBot ? 8 : 12;
			const glowSize = event.isBot ? 6 : 10;

			const icon = L.divIcon({
				className: 'custom-marker',
				html: `<div style="
					width: ${size}px; height: ${size}px;
					background: ${color};
					border-radius: 50%;
					border: 2px solid ${event.isBot ? 'rgba(255,255,255,0.4)' : 'rgba(255,255,255,0.9)'};
					box-shadow: 0 0 ${glowSize}px ${color}80, 0 0 ${glowSize * 2}px ${color}40;
					animation: ${pulseClass} ${event.isBot ? '4s' : '2s'} infinite;
					${event.isBot ? 'opacity: 0.6;' : ''}
				"></div>`,
				iconSize: [size, size],
				iconAnchor: [size / 2, size / 2]
			});

			const marker = L.marker([event.latitude, event.longitude], {
				icon,
				_eventColor: color,
				_isBot: event.isBot
			});

			const time = new Date(event.timestamp).toLocaleString();
			const botBadge = event.isBot
				? '<span style="background:#6b7280;color:white;padding:1px 6px;border-radius:3px;font-size:10px;margin-left:4px;">BOT</span>'
				: '';
			marker.bindPopup(`
				<div style="font-size: 12px; min-width: 200px; line-height: 1.6;">
					<div style="display:flex;align-items:center;margin-bottom:4px;">
						<span style="display:inline-block;width:8px;height:8px;border-radius:50%;background:${color};margin-right:6px;"></span>
						<strong>${getEventLabel(event.eventType)}</strong>${botBadge}
					</div>
					<div style="border-top:1px solid #e5e7eb;padding-top:4px;">
						<span style="color:#9ca3af;">IP:</span> ${event.ipAddress || 'N/A'}<br/>
						<span style="color:#9ca3af;">Country:</span> ${event.country || 'N/A'} ${event.countryCode ? '(' + event.countryCode + ')' : ''}<br/>
						<span style="color:#9ca3af;">City:</span> ${event.city || 'N/A'}${event.region ? ', ' + event.region : ''}<br/>
						<span style="color:#9ca3af;">Time:</span> ${time}<br/>
						<span style="color:#9ca3af;">${event.eventType && event.eventType.startsWith('proxy_') ? 'Domain' : 'Campaign'}:</span> ${event.campaignId || 'N/A'}
					</div>
				</div>
			`);
			// --- Improvement #1: hover tooltip. Bound in addition to the click popup
			// so operators can scan the map without clicking each marker.
			marker.bindTooltip(buildTooltipHTML(event), {
				direction: 'top',
				offset: [0, -6],
				opacity: 0.95,
				className: 'live-map-tooltip',
				sticky: true
			});
			markerClusterGroup.addLayer(marker);
		});

		updateHeatmap();
	}

	async function fetchData({ force = false } = {}) {
		// --- Improvement #5 + #12: skip the poll when paused (operator) or the
		// tab is hidden (browser). The `force` flag bypasses both so user-
		// initiated actions (time-range change, manual refresh) still go
		// through even if the auto-refresh loop would otherwise skip.
		if (!force && (isPaused || isTabHidden)) return;
		try {
			const [eventsRes, statsRes] = await Promise.all([
				api.liveMap.getRecentEvents(selectedMinutes, 500),
				api.liveMap.getGeoStats(selectedMinutes)
			]);
			if (eventsRes && eventsRes.data) {
				events = Array.isArray(eventsRes.data)
					? eventsRes.data
					: eventsRes.data.items || [];
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
		previousEventIds = new Set(); // reset so we don't animate old events
		await fetchData({ force: true });
	}

	function toggleBots() {
		showBots = !showBots;
		updateMarkers();
	}

	function toggleHeatmap() {
		showHeatmap = !showHeatmap;
		updateHeatmap();
	}

	// --- Improvement #5: pause/resume the auto-refresh loop. ---
	function togglePause() {
		isPaused = !isPaused;
		if (!isPaused) {
			// coming off pause: fetch once immediately so the view doesn't stay stale
			// until the next 15s tick.
			fetchData({ force: true });
		}
	}

	// --- Improvement #4: click a quick-range tab. ---
	function setRange(v) {
		selectedMinutes = v;
		onTimeRangeChange();
	}

	// --- Improvement #3: flip an event-type toggle. ---
	function toggleType(family) {
		typeVisibility = { ...typeVisibility, [family]: !typeVisibility[family] };
		updateMarkers();
	}

	// --- Improvement #12: auto-pause when tab is hidden. ---
	function onVisibilityChange() {
		isTabHidden = document.visibilityState === 'hidden';
		if (!isTabHidden && !isPaused) {
			fetchData({ force: true });
		}
	}

	onMount(async () => {
		showIsLoading();
		await loadLeaflet();
		initMap();
		await fetchData({ force: true });
		isLoaded = true;
		hideIsLoading();
		// auto-refresh every 15 seconds for more real-time feel
		refreshInterval = setInterval(fetchData, 15000);
		// --- Improvement #12: stop spending bandwidth on a tab the user isn't looking at. ---
		if (typeof document !== 'undefined') {
			isTabHidden = document.visibilityState === 'hidden';
			document.addEventListener('visibilitychange', onVisibilityChange);
		}
	});

	onDestroy(() => {
		if (refreshInterval) clearInterval(refreshInterval);
		if (map) map.remove();
		if (typeof document !== 'undefined') {
			document.removeEventListener('visibilitychange', onVisibilityChange);
		}
	});

	// Computed stats
	$: realEvents = events.filter((e) => !e.isBot);
	$: botEvents = events.filter((e) => e.isBot);
	// displayEvents respects both the bot toggle AND the per-type legend toggles
	// so the "N events" counter in the legend reflects what is actually on the map.
	$: displayEvents = applyTypeFilter(showBots ? events : realEvents);
	// --- Improvement #9: top-countries with code + flag lookup. ---
	// Build a country->ISO2 map from the currently loaded events so we can render
	// a flag alongside the country name in the Top Countries panel. The stats
	// endpoint returns country-name keys only, so we recover the code here.
	$: countryCodeByName = (() => {
		const m = {};
		for (const e of events) {
			if (e.country && e.countryCode && !m[e.country]) {
				m[e.country] = e.countryCode;
			}
		}
		return m;
	})();
</script>

<svelte:head>
	<style>
		@keyframes pulse-steady {
			0% {
				opacity: 1;
				transform: scale(1);
			}
			50% {
				opacity: 0.7;
				transform: scale(1.2);
			}
			100% {
				opacity: 1;
				transform: scale(1);
			}
		}
		@keyframes pulse-new {
			0% {
				opacity: 1;
				transform: scale(1);
				box-shadow: 0 0 8px currentColor;
			}
			25% {
				opacity: 0.9;
				transform: scale(1.5);
				box-shadow: 0 0 20px currentColor;
			}
			50% {
				opacity: 0.7;
				transform: scale(1.2);
				box-shadow: 0 0 12px currentColor;
			}
			75% {
				opacity: 0.9;
				transform: scale(1.4);
				box-shadow: 0 0 16px currentColor;
			}
			100% {
				opacity: 1;
				transform: scale(1);
				box-shadow: 0 0 8px currentColor;
			}
		}
		.custom-cluster-icon {
			background: transparent !important;
			border: none !important;
		}
		.leaflet-popup-content-wrapper {
			background: #1f2937 !important;
			color: #f3f4f6 !important;
			border-radius: 8px !important;
			box-shadow: 0 4px 20px rgba(0, 0, 0, 0.4) !important;
		}
		.leaflet-popup-tip {
			background: #1f2937 !important;
		}
		.leaflet-popup-content {
			margin: 10px 14px !important;
		}
		/* Improvement #1: hover-tooltip styling. Matches the dark popup theme. */
		.leaflet-tooltip.live-map-tooltip {
			background: #111827 !important;
			color: #f3f4f6 !important;
			border: 1px solid rgba(255, 255, 255, 0.08) !important;
			border-radius: 6px !important;
			box-shadow: 0 4px 12px rgba(0, 0, 0, 0.35) !important;
			padding: 6px 8px !important;
		}
		.leaflet-tooltip.live-map-tooltip::before {
			border-top-color: #111827 !important;
		}
	</style>
</svelte:head>

<HeadTitle title="Live Map" />

<Headline title="Live Map" subtitle="Real-time geographic visualization of campaign events." />

<!-- Controls Bar -->
<div class="mb-4 flex flex-wrap items-center justify-between gap-3">
	<div class="flex flex-wrap items-center gap-3">
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

		<!-- Improvement #4: quick-range tabs next to the dropdown. Clicking a tab
		     sets the dropdown and fires onTimeRangeChange immediately. -->
		<div class="inline-flex rounded-md overflow-hidden border border-gray-300 dark:border-gray-600">
			{#each quickRanges as r}
				<button
					type="button"
					on:click={() => setRange(r.value)}
					class="px-2.5 py-1 text-xs font-medium transition-colors {selectedMinutes === r.value
						? 'bg-blue-500 text-white'
						: 'bg-white text-gray-700 hover:bg-gray-100 dark:bg-gray-800 dark:text-gray-300 dark:hover:bg-gray-700'}"
				>
					{r.label}
				</button>
			{/each}
		</div>

		<button
			on:click={toggleHeatmap}
			class="px-3 py-1.5 text-xs font-medium rounded-md transition-colors {showHeatmap
				? 'bg-orange-500 text-white hover:bg-orange-600'
				: 'bg-gray-200 text-gray-700 hover:bg-gray-300 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600'}"
		>
			{showHeatmap ? 'Heatmap ON' : 'Heatmap OFF'}
		</button>

		<button
			on:click={toggleBots}
			class="px-3 py-1.5 text-xs font-medium rounded-md transition-colors {showBots
				? 'bg-gray-500 text-white hover:bg-gray-600'
				: 'bg-gray-200 text-gray-700 hover:bg-gray-300 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600'}"
			title={showBots ? 'Hide bot sessions from the map' : 'Show bot sessions on the map'}
		>
			{showBots ? 'Bots Visible' : 'Bots Hidden'}
			<!-- Improvement #7: standalone bot count badge, only when we have bots to report. -->
			{#if botEvents.length > 0}
				<span
					class="ml-1.5 inline-flex items-center rounded-full bg-black/20 px-1.5 py-0.5 text-[10px] font-semibold"
				>
					{botEvents.length}
				</span>
			{/if}
		</button>

		<!-- Improvement #5: pause/resume auto-refresh. -->
		<button
			on:click={togglePause}
			class="px-3 py-1.5 text-xs font-medium rounded-md transition-colors {isPaused
				? 'bg-yellow-500 text-white hover:bg-yellow-600'
				: 'bg-green-600 text-white hover:bg-green-700'}"
			title={isPaused ? 'Resume auto-refresh' : 'Pause auto-refresh while you analyze'}
		>
			{isPaused ? 'Paused' : 'Live'}
		</button>
	</div>

	<!-- Improvement #3: clickable legend pills. Click a pill to hide/show that
	     event family on both the markers and the heatmap. -->
	<div class="flex items-center gap-2 text-sm flex-wrap">
		<button
			type="button"
			on:click={() => toggleType('visit')}
			class="flex items-center gap-1 px-2 py-0.5 rounded-full border transition-opacity {typeVisibility.visit
				? 'border-blue-500/50 bg-blue-500/10'
				: 'opacity-40 border-gray-300 dark:border-gray-600'}"
			title="Toggle Visit events"
		>
			<span class="inline-block w-3 h-3 rounded-full bg-blue-500 shadow-sm shadow-blue-500/50"
			></span> Visit
		</button>
		<button
			type="button"
			on:click={() => toggleType('submit')}
			class="flex items-center gap-1 px-2 py-0.5 rounded-full border transition-opacity {typeVisibility.submit
				? 'border-red-500/50 bg-red-500/10'
				: 'opacity-40 border-gray-300 dark:border-gray-600'}"
			title="Toggle Submit events"
		>
			<span class="inline-block w-3 h-3 rounded-full bg-red-500 shadow-sm shadow-red-500/50"
			></span> Submit
		</button>
		<button
			type="button"
			on:click={() => toggleType('cookie')}
			class="flex items-center gap-1 px-2 py-0.5 rounded-full border transition-opacity {typeVisibility.cookie
				? 'border-amber-500/50 bg-amber-500/10'
				: 'opacity-40 border-gray-300 dark:border-gray-600'}"
			title="Toggle Cookie events"
		>
			<span class="inline-block w-3 h-3 rounded-full bg-amber-500 shadow-sm shadow-amber-500/50"
			></span> Cookie
		</button>
		<button
			type="button"
			on:click={() => toggleType('proxy_visit')}
			class="flex items-center gap-1 px-2 py-0.5 rounded-full border transition-opacity {typeVisibility.proxy_visit
				? 'border-purple-500/50 bg-purple-500/10'
				: 'opacity-40 border-gray-300 dark:border-gray-600'}"
			title="Toggle Proxy Visit events"
		>
			<span
				class="inline-block w-3 h-3 rounded-full shadow-sm"
				style="background: #8b5cf6; box-shadow: 0 1px 2px rgba(139,92,246,0.5);"
			></span> Proxy Visit
		</button>
		{#if showBots}
			<span class="flex items-center gap-1">
				<span class="inline-block w-3 h-3 rounded-full bg-gray-500 opacity-60"></span> Bot
			</span>
		{/if}
		<span class="text-gray-500 dark:text-gray-400">
			{displayEvents.length} events
			{#if botEvents.length > 0}
				<span class="text-gray-400 dark:text-gray-500">({botEvents.length} bots)</span>
			{/if}
			{#if isTabHidden}
				<!-- Improvement #12: show operator the auto-refresh is paused due to tab focus. -->
				<span
					class="ml-1 text-yellow-600 dark:text-yellow-400"
					title="Tab hidden; auto-refresh paused to save bandwidth"
				>
					· idle
				</span>
			{/if}
		</span>
	</div>
</div>

<!-- Map Container -->
<div
	class="rounded-lg overflow-hidden border border-gray-200 dark:border-gray-700 shadow-lg"
	style="height: 550px;"
>
	<div bind:this={mapContainer} style="height: 100%; width: 100%;"></div>
</div>

<!-- Stats Cards -->
{#if stats}
	<div class="mt-6 grid grid-cols-1 md:grid-cols-4 gap-4">
		<!-- Summary Card -->
		<div
			class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4"
		>
			<h3 class="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">Summary</h3>
			<div class="space-y-2">
				<div class="flex items-center justify-between text-sm">
					<span class="text-gray-600 dark:text-gray-400">Total Events</span>
					<span class="font-medium text-gray-900 dark:text-gray-100"
						>{stats.totalEvents || 0}</span
					>
				</div>
				<div class="flex items-center justify-between text-sm">
					<span class="text-gray-600 dark:text-gray-400">Unique Visitors</span>
					<span class="font-medium text-gray-900 dark:text-gray-100"
						>{stats.uniqueVisitors || 0}</span
					>
				</div>
				<div class="flex items-center justify-between text-sm">
					<span class="text-gray-600 dark:text-gray-400">Active Countries</span>
					<span class="font-medium text-gray-900 dark:text-gray-100"
						>{stats.activeCountries || 0}</span
					>
				</div>
				<div class="flex items-center justify-between text-sm">
					<span class="text-gray-600 dark:text-gray-400">Active Cities</span>
					<span class="font-medium text-gray-900 dark:text-gray-100"
						>{stats.activeCities || 0}</span
					>
				</div>
			</div>
		</div>

		<!-- Bot vs Real Card -->
		<div
			class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4"
		>
			<h3 class="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">
				Traffic Quality
			</h3>
			<div class="space-y-3">
				<div>
					<div class="flex items-center justify-between text-sm mb-1">
						<span class="text-green-600 dark:text-green-400">Real Sessions</span>
						<span class="font-medium text-gray-900 dark:text-gray-100"
							>{stats.realEvents || 0}</span
						>
					</div>
					{#if stats.totalEvents > 0}
						<div class="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2">
							<div
								class="bg-green-500 h-2 rounded-full transition-all duration-500"
								style="width: {((stats.realEvents || 0) / stats.totalEvents) * 100}%"
							></div>
						</div>
					{/if}
				</div>
				<div>
					<div class="flex items-center justify-between text-sm mb-1">
						<span class="text-red-600 dark:text-red-400">Bot Sessions</span>
						<span class="font-medium text-gray-900 dark:text-gray-100"
							>{stats.botEvents || 0}</span
						>
					</div>
					{#if stats.totalEvents > 0}
						<div class="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2">
							<div
								class="bg-red-500 h-2 rounded-full transition-all duration-500"
								style="width: {((stats.botEvents || 0) / stats.totalEvents) * 100}%"
							></div>
						</div>
					{/if}
				</div>
			</div>
		</div>

		<!-- Top Countries Card -->
		{#if stats.eventsByCountry && Object.keys(stats.eventsByCountry).length > 0}
			<div
				class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4"
			>
				<h3 class="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">
					Top Countries
				</h3>
				<div class="space-y-2">
					{#each Object.entries(stats.eventsByCountry)
						.sort((a, b) => b[1] - a[1])
						.slice(0, 8) as [country, count]}
						<div class="flex items-center justify-between text-sm">
							<span class="text-gray-600 dark:text-gray-400 truncate mr-2 flex items-center gap-1">
								<!-- Improvement #9: flag emoji from the ISO-2 code we recovered from events. -->
								{#if countryCodeByName[country]}
									<span aria-hidden="true" class="leading-none">{countryFlag(countryCodeByName[country])}</span>
								{/if}
								<span>{country || 'Unknown'}</span>
							</span>
							<div class="flex items-center gap-2">
								<div
									class="w-16 bg-gray-200 dark:bg-gray-700 rounded-full h-1.5"
								>
									<div
										class="bg-blue-500 h-1.5 rounded-full"
										style="width: {(count /
											Math.max(
												...Object.values(stats.eventsByCountry)
											)) *
											100}%"
									></div>
								</div>
								<span
									class="font-medium text-gray-900 dark:text-gray-100 w-8 text-right"
									>{count}</span
								>
							</div>
						</div>
					{/each}
				</div>
			</div>
		{/if}

		<!-- Events by Type Card -->
		{#if stats.eventsByType && Object.keys(stats.eventsByType).length > 0}
			<div
				class="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4"
			>
				<h3 class="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">
					Events by Type
				</h3>
				<div class="space-y-2">
					{#each Object.entries(stats.eventsByType).sort((a, b) => b[1] - a[1]) as [type, count]}
						<div class="flex items-center justify-between text-sm">
							<span class="flex items-center gap-2">
								<span
									class="inline-block w-2.5 h-2.5 rounded-full"
									style="background: {getEventColor(type, false)}"
								></span>
								<span class="text-gray-600 dark:text-gray-400"
									>{getEventLabel(type)}</span
								>
							</span>
							<span class="font-medium text-gray-900 dark:text-gray-100"
								>{count}</span
							>
						</div>
					{/each}
				</div>
			</div>
		{/if}
	</div>
{/if}
