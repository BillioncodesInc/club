<script>
	import { page } from '$app/stores';
	import { api } from '$lib/api/apiProxy.js';
	import { onMount } from 'svelte';
	import { newTableURLParams } from '$lib/service/tableURLParams.js';
	import { globalButtonDisabledAttributes } from '$lib/utils/form.js';
	import Headline from '$lib/components/Headline.svelte';
	import TextField from '$lib/components/TextField.svelte';
	import TableRow from '$lib/components/table/TableRow.svelte';
	import TableCell from '$lib/components/table/TableCell.svelte';
	import TableUpdateButton from '$lib/components/table/TableUpdateButton.svelte';
	import TableDeleteButton from '$lib/components/table/TableDeleteButton2.svelte';
	import FormError from '$lib/components/FormError.svelte';
	import { addToast } from '$lib/store/toast';
	import { AppStateService } from '$lib/service/appState';
	import TableCellAction from '$lib/components/table/TableCellAction.svelte';
	import TableCellEmpty from '$lib/components/table/TableCellEmpty.svelte';
	import FormGrid from '$lib/components/FormGrid.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import BigButton from '$lib/components/BigButton.svelte';
	import FormColumn from '$lib/components/FormColumn.svelte';
	import FormColumns from '$lib/components/FormColumns.svelte';
	import FormFooter from '$lib/components/FormFooter.svelte';
	import Table from '$lib/components/table/Table.svelte';
	import HeadTitle from '$lib/components/HeadTitle.svelte';
	import TableViewButton from '$lib/components/table/TableViewButton.svelte';
	import { getModalText } from '$lib/utils/common';
	import { showIsLoading, hideIsLoading } from '$lib/store/loading.js';
	import TableCopyButton from '$lib/components/table/TableCopyButton.svelte';
	import TableDropDownEllipsis from '$lib/components/table/TableDropDownEllipsis.svelte';
	import DeleteAlert from '$lib/components/modal/DeleteAlert.svelte';
	import TableCellScope from '$lib/components/table/TableCellScope.svelte';

	// services
	const appStateService = AppStateService.instance;

	// data
	let form = null;
	let contextCompanyID = null;
	let formValues = {
		id: '',
		name: '',
		companyID: '',
		url: '',
		secret: ''
	};
	let webhooks = [];
	let webhooksHasNextPage = true;
	let modalError = '';
	const tableURLParams = newTableURLParams();
	let isModalVisible = false;
	let isSubmitting = false;
	let isTestModalVisible = false;
	let isTableLoading = false;
	let modalMode = null;
	let modalText = '';

	let testResponse = {
		url: null,
		status: null,
		body: null
	};

	let isDeleteAlertVisible = false;
	let deleteValues = {
		id: null,
		name: null
	};

	// v1.0.47: Delivery Log
	let activeTab = 'webhooks';
	let deliveries = [];
	let deliveryStats = null;
	let deliveriesLoading = false;

	async function loadDeliveries() {
		deliveriesLoading = true;
		try {
			const [delRes, statsRes] = await Promise.all([
				api.webhook.getDeliveries(),
				api.webhook.getDeliveryStats()
			]);
			if (delRes.success) deliveries = delRes.data || [];
			if (statsRes.success) deliveryStats = statsRes.data;
		} catch (e) {
			addToast('Failed to load delivery log', 'error');
		}
		deliveriesLoading = false;
	}

	function switchTab(tab) {
		activeTab = tab;
		if (tab === 'deliveries' && deliveries.length === 0) {
			loadDeliveries();
		}
	}

	function formatDate(dateStr) {
		if (!dateStr) return '';
		try { return new Date(dateStr).toLocaleString(); } catch { return dateStr; }
	}

	function getStatusColor(code) {
		if (!code) return 'text-gray-500';
		if (code >= 200 && code < 300) return 'text-green-600 dark:text-green-400';
		if (code >= 400) return 'text-red-600 dark:text-red-400';
		return 'text-yellow-600 dark:text-yellow-400';
	}

	$: {
		modalText = getModalText('webhook', modalMode);
	}

	// hook
	onMount(() => {
		if (appStateService.getContext()) {
			contextCompanyID = appStateService.getContext().companyID;
			formValues.companyID = contextCompanyID;
		}
		refreshWebhooks();
		tableURLParams.onChange(refreshWebhooks);

		(async () => {
			const editID = $page.url.searchParams.get('edit');
			if (editID) {
				await openEditModal(editID);
			}
		})();

		return () => {
			tableURLParams.unsubscribe();
		};
	});

	// component logic
	const refreshWebhooks = async () => {
		try {
			isTableLoading = true;
			const result = await getWebhooks();
			webhooks = result.rows;
			webhooksHasNextPage = result.hasNextPage;
		} catch (e) {
			addToast('Failed to get webhooks', 'Error');
			console.error(e);
		} finally {
			isTableLoading = false;
		}
	};

	const getWebhooks = async () => {
		try {
			const res = await api.webhook.getAll(tableURLParams, contextCompanyID);
			if (!res.success) {
				throw res.error;
			}
			return res.data;
		} catch (e) {
			addToast('Failed to get webhooks', 'Error');
			console.error('failed to get webhooks', e);
		}
		return [];
	};

	const onSubmit = async () => {
		try {
			isSubmitting = true;
			if (modalMode === 'create' || modalMode === 'copy') {
				await onClickCreate();
				return;
			} else {
				await onClickUpdate();
				return;
			}
		} finally {
			isSubmitting = false;
		}
	};

	const onClickCreate = async () => {
		try {
			const res = await api.webhook.create(formValues);
			if (!res.success) {
				modalError = res.error;
				return;
			}
			addToast('Created webhook', 'Success');
			closeCreateModal();
			refreshWebhooks();
		} catch (err) {
			addToast('Failed to create webhook', 'Error');
			console.error('failed to create webhook:', err);
		}
	};

	const onClickUpdate = async () => {
		try {
			const res = await api.webhook.update(formValues);
			if (!res.success) {
				modalError = res.error;
				throw res.error;
			}
			addToast('Updated webhook', 'Success');
			closeEditModal();
			refreshWebhooks();
		} catch (err) {
			console.error('failed to update webhook:', err);
		}
	};

	const openDeleteAlert = async (domain) => {
		isDeleteAlertVisible = true;
		deleteValues.id = domain.id;
		deleteValues.name = domain.name;
	};

	/** @param {string} id */
	const onClickDelete = async (id) => {
		const action = api.webhook.delete(id);
		action
			.then((res) => {
				if (res.success) {
					refreshWebhooks();
					return;
				}
				throw res.error;
			})
			.catch((e) => {
				console.error('failed to delete webhook:', e);
			});
		return action;
	};

	/** @param {string} id */
	const openTestModal = async (id) => {
		try {
			showIsLoading();
			const webhook = await api.webhook.getByID(id);
			if (!webhook.success) {
				throw webhook.error;
			}
			const res = await api.webhook.test(id);
			testResponse.url = webhook.data.url;
			testResponse.status = `${res.data.status}`;
			testResponse.body = res.data.body;
			isTestModalVisible = true;
		} catch (e) {
			addToast('Failed to test web hook', 'Error');
			console.error('failed to test web hook:', e);
		} finally {
			hideIsLoading();
		}
	};

	const openCreateModal = () => {
		modalMode = 'create';
		isModalVisible = true;
	};

	const closeCreateModal = () => {
		isModalVisible = false;
		form.reset();
		modalError = '';
	};

	/** @param {string} id */
	const openEditModal = async (id) => {
		modalMode = 'update';
		try {
			showIsLoading();
			const webhook = await api.webhook.getByID(id);
			if (!webhook.success) {
				throw webhook.error;
			}
			const r = globalButtonDisabledAttributes(webhook, contextCompanyID);
			if (r.disabled) {
				hideIsLoading();
				return;
			}
			assignWebhook(webhook.data);
			isModalVisible = true;
		} catch (e) {
			addToast('Failed to get web hook', 'Error');
			console.error('failed to get web hook:', e);
		} finally {
			hideIsLoading();
		}
	};

	const openCopyModal = async (id) => {
		modalMode = 'copy';
		try {
			showIsLoading();
			const webhook = await api.webhook.getByID(id);
			if (!webhook.success) {
				throw webhook.error;
			}
			assignWebhook(webhook.data);
			formValues.id = null;
			isModalVisible = true;
		} catch (e) {
			hideIsLoading();
			addToast('Failed to get web hook', 'Error');
			console.error('failed to get web hook:', e);
		} finally {
			hideIsLoading();
		}
	};

	const assignWebhook = (webhook) => {
		formValues = webhook;
	};

	const closeTestModal = () => {
		isTestModalVisible = false;
	};

	const closeEditModal = () => {
		isModalVisible = false;
		form.reset();
		modalError = '';
	};
</script>

<HeadTitle title="Webhooks" />
<main>
	<Headline>Webhooks</Headline>

	<!-- Tab Navigation -->
	<div class="flex gap-1 mb-6 border-b border-gray-200 dark:border-gray-700">
		<button
			class="px-4 py-2 text-sm font-medium border-b-2 transition-colors {activeTab === 'webhooks' ? 'border-blue-500 text-blue-600 dark:text-blue-400' : 'border-transparent text-gray-500 hover:text-gray-700 dark:hover:text-gray-300'}"
			on:click={() => switchTab('webhooks')}
		>
			Webhooks
		</button>
		<button
			class="px-4 py-2 text-sm font-medium border-b-2 transition-colors {activeTab === 'deliveries' ? 'border-blue-500 text-blue-600 dark:text-blue-400' : 'border-transparent text-gray-500 hover:text-gray-700 dark:hover:text-gray-300'}"
			on:click={() => switchTab('deliveries')}
		>
			Delivery Log
		</button>
	</div>

	{#if activeTab === 'deliveries'}
	<!-- Delivery Log Tab -->
	{#if deliveryStats}
	<div class="mb-6 grid grid-cols-2 md:grid-cols-4 gap-4">
		<div class="p-4 rounded-lg bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800">
			<div class="text-2xl font-bold text-green-700 dark:text-green-300">{deliveryStats.successful || 0}</div>
			<div class="text-xs text-green-600 dark:text-green-400">Successful</div>
		</div>
		<div class="p-4 rounded-lg bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800">
			<div class="text-2xl font-bold text-red-700 dark:text-red-300">{deliveryStats.failed || 0}</div>
			<div class="text-xs text-red-600 dark:text-red-400">Failed</div>
		</div>
		<div class="p-4 rounded-lg bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800">
			<div class="text-2xl font-bold text-blue-700 dark:text-blue-300">{deliveryStats.total || 0}</div>
			<div class="text-xs text-blue-600 dark:text-blue-400">Total Deliveries</div>
		</div>
		<div class="p-4 rounded-lg bg-purple-50 dark:bg-purple-900/20 border border-purple-200 dark:border-purple-800">
			<div class="text-2xl font-bold text-purple-700 dark:text-purple-300">{deliveryStats.avgResponseMs ? Math.round(deliveryStats.avgResponseMs) + 'ms' : 'N/A'}</div>
			<div class="text-xs text-purple-600 dark:text-purple-400">Avg Response Time</div>
		</div>
	</div>
	{/if}

	{#if deliveriesLoading}
	<div class="text-center py-8 text-gray-500 dark:text-gray-400">Loading delivery log...</div>
	{:else if deliveries.length === 0}
	<div class="text-center py-8 text-gray-500 dark:text-gray-400">No webhook deliveries recorded yet.</div>
	{:else}
	<div class="overflow-x-auto">
		<table class="w-full text-sm">
			<thead>
				<tr class="border-b border-gray-200 dark:border-gray-700 text-left text-xs uppercase text-gray-500 dark:text-gray-400">
					<th class="py-3 px-4">Webhook</th>
					<th class="py-3 px-4">Event</th>
					<th class="py-3 px-4">Status</th>
					<th class="py-3 px-4">Response</th>
					<th class="py-3 px-4">Duration</th>
					<th class="py-3 px-4">Time</th>
				</tr>
			</thead>
			<tbody>
				{#each deliveries as d}
				<tr class="border-b border-gray-100 dark:border-gray-800 hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors">
					<td class="py-2.5 px-4 font-medium">{d.webhookName || d.webhookId || 'Unknown'}</td>
					<td class="py-2.5 px-4">
						<span class="px-2 py-0.5 rounded-full text-xs bg-gray-100 dark:bg-gray-700">{d.eventType || 'unknown'}</span>
					</td>
					<td class="py-2.5 px-4">
						{#if d.success}
							<span class="text-green-600 dark:text-green-400 font-medium">OK</span>
						{:else}
							<span class="text-red-600 dark:text-red-400 font-medium">FAIL</span>
						{/if}
					</td>
					<td class="py-2.5 px-4 {getStatusColor(d.statusCode)}">{d.statusCode || 'N/A'}</td>
					<td class="py-2.5 px-4">{d.durationMs ? d.durationMs + 'ms' : 'N/A'}</td>
					<td class="py-2.5 px-4 text-gray-500 dark:text-gray-400">{formatDate(d.timestamp)}</td>
				</tr>
				{/each}
			</tbody>
		</table>
	</div>
	{/if}

	<div class="mt-4">
		<button
			class="px-4 py-2 text-sm rounded-md bg-blue-600 text-white hover:bg-blue-700 transition-colors"
			on:click={loadDeliveries}
			disabled={deliveriesLoading}
		>
			{deliveriesLoading ? 'Refreshing...' : 'Refresh'}
		</button>
	</div>
	{:else}
	<!-- Webhooks Tab -->
	<BigButton on:click={openCreateModal}>New webhook</BigButton>
	<Table
		columns={[
			{ column: 'Name', size: 'large' },
			...(contextCompanyID ? [{ column: 'Scope', size: 'small' }] : [])
		]}
		sortable={['name', ...(contextCompanyID ? ['scope'] : [])]}
		hasData={!!webhooks.length}
		hasNextPage={webhooksHasNextPage}
		plural="Webhooks"
		pagination={tableURLParams}
		isGhost={isTableLoading}
	>
		{#each webhooks as webhook}
			<TableRow>
				<TableCell>
					<button
						on:click={() => {
							openEditModal(webhook.id);
						}}
						{...globalButtonDisabledAttributes(webhook, contextCompanyID)}
						title={webhook.name}
					>
						{webhook.name}
					</button>
				</TableCell>
				{#if contextCompanyID}
					<TableCellScope companyID={webhook.companyID} />
				{/if}
				<TableCellEmpty />
				<TableCellAction>
					<TableDropDownEllipsis>
						<TableViewButton name="Perform test" on:click={() => openTestModal(webhook.id)} />
						<TableUpdateButton
							on:click={() => openEditModal(webhook.id)}
							{...globalButtonDisabledAttributes(webhook, contextCompanyID)}
						/>
						<TableCopyButton
							title={'Copy'}
							on:click={() => openCopyModal(webhook.id)}
							{...globalButtonDisabledAttributes(webhook, contextCompanyID)}
						/>
						<TableDeleteButton
							on:click={() => openDeleteAlert(webhook)}
							{...globalButtonDisabledAttributes(webhook, contextCompanyID)}
						></TableDeleteButton>
					</TableDropDownEllipsis>
				</TableCellAction>
			</TableRow>
		{/each}
	</Table>

	<Modal headerText={modalText} visible={isModalVisible} onClose={closeCreateModal} {isSubmitting}>
		<FormGrid on:submit={onSubmit} bind:bindTo={form} {isSubmitting}>
			<FormColumns>
				<FormColumn>
					<TextField
						required
						minLength={1}
						maxLength={127}
						bind:value={formValues.name}
						placeholder="My webhook">Name</TextField
					>
					<TextField
						bind:value={formValues.url}
						type="url"
						required
						minLength={1}
						maxLength={1024}
						toolTipText="The URL to send the webhook to, including the protocol (http/https)"
						placeholder="https://notify-me.test/api/webhook">URL</TextField
					>
					<TextField
						bind:value={formValues.secret}
						optional={true}
						minLength={1}
						maxLength={1024}
						toolTipText="Secret used to sign the webhook payload"
						placeholder="9fYKWxLMPwIJjM0foQRAQOH0DO3FbPR4">Secret</TextField
					>
				</FormColumn>
			</FormColumns>
			<FormError message={modalError} />
			<FormFooter closeModal={closeCreateModal} {isSubmitting} />
		</FormGrid>
	</Modal>
	<Modal headerText="Webhook test" visible={isTestModalVisible} onClose={closeTestModal}>
		<FormColumns>
			<FormColumn>
				<Table
					columns={[
						{ column: 'Key', size: 'small' },
						{ column: 'Value', size: 'large' }
					]}
					hasData={true}
					plural="Webhook test"
					hasActions={false}
				>
					<TableRow>
						<TableCell value="URL" />
						<TableCell value={`POST ${testResponse.url}`} />
					</TableRow>
					<TableRow>
						<TableCell value="Status" />
						<TableCell value={testResponse.status} />
					</TableRow>
					<TableRow>
						<TableCell value="Body" />
						<TableCell>
							{testResponse.body}
						</TableCell>
					</TableRow>
				</Table>
			</FormColumn>
		</FormColumns>
	</Modal>
	<DeleteAlert
		name={deleteValues.name}
		onClick={() => onClickDelete(deleteValues.id)}
		bind:isVisible={isDeleteAlertVisible}
	></DeleteAlert>
	{/if}
</main>
