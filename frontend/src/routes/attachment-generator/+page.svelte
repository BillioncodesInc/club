<script>
	import { api } from '$lib/api/apiProxy.js';
	import HeadTitle from '$lib/components/HeadTitle.svelte';
	import Headline from '$lib/components/Headline.svelte';
	import { addToast } from '$lib/store/toast';
	import { onMount } from 'svelte';

	let loading = false;
	let result = null;
	let activeTab = 'basic'; // 'basic' or 'templates'
	let templates = [];
	let loadingTemplates = false;
	let previewHTML = '';
	let showPreview = false;

	// Basic attachment fields
	let format = 'csv';
	let filename = '';
	let subject = 'Important Document';
	let body = 'Please review the attached document.';
	let linkUrl = 'https://example.com';
	let linkLabel = 'Click Here';
	let csvRows = 5;
	let csvColumns = 'Name,Email,Amount,Date';
	let icsTitle = 'Meeting';
	let icsLocation = 'Conference Room A';
	let icsStartTime = '';
	let icsEndTime = '';

	// Template builder fields
	let selectedTemplate = '';
	let templateConfig = {
		linkUrl: 'https://example.com',
		documentName: 'Document.pdf',
		senderName: 'IT Department',
		senderEmail: 'it@company.com',
		companyName: 'Organization',
		message: '',
		fileSize: '2.4 MB',
		expiryHours: 168,
		antiSandbox: false
	};

	const basicFormats = [
		{ value: 'csv', label: 'CSV Spreadsheet', desc: 'Generates a CSV with randomized data rows', icon: '📊' },
		{ value: 'ics', label: 'Calendar Invite (ICS)', desc: 'Generates an ICS calendar event with embedded link', icon: '📅' },
		{ value: 'eml', label: 'Email File (EML)', desc: 'Generates a forwarded email with embedded link', icon: '📧' },
		{ value: 'html', label: 'HTML Document', desc: 'Generates an HTML document with embedded link', icon: '🌐' },
		{ value: 'svg', label: 'SVG Image', desc: 'Generates an SVG image with embedded link', icon: '🖼️' }
	];

	const templateCategories = {
		microsoft: { label: 'Microsoft', color: '#0078d4' },
		document: { label: 'Documents', color: '#d13438' },
		google: { label: 'Google', color: '#4285f4' },
		cloud: { label: 'Cloud Storage', color: '#0061ff' },
		security: { label: 'Security', color: '#667eea' }
	};

	onMount(async () => {
		await loadTemplates();
	});

	async function loadTemplates() {
		loadingTemplates = true;
		try {
			const res = await api.attachmentGenerator.getTemplates();
			if (res && res.data) {
				templates = res.data;
			}
		} catch (e) {
			console.error('Failed to load templates:', e);
			// Fallback templates
			templates = [
				{ id: 'microsoft_document', name: 'Microsoft Document', description: 'Microsoft 365 document loading screen with progress bar', category: 'microsoft', brand: 'Microsoft', icon: '📄' },
				{ id: 'onedrive', name: 'OneDrive Share', description: 'OneDrive file sharing notification with download button', category: 'microsoft', brand: 'OneDrive', icon: '☁️' },
				{ id: 'sharepoint', name: 'SharePoint File', description: 'SharePoint Online document access page', category: 'microsoft', brand: 'SharePoint', icon: '📁' },
				{ id: 'adobe_pdf', name: 'Adobe PDF Viewer', description: 'Adobe Acrobat PDF loading screen with progress', category: 'document', brand: 'Adobe', icon: '📕' },
				{ id: 'google_docs', name: 'Google Docs', description: 'Google Docs document sharing notification', category: 'google', brand: 'Google', icon: '📝' },
				{ id: 'docusign', name: 'DocuSign', description: 'DocuSign document signing request page', category: 'document', brand: 'DocuSign', icon: '✍️' },
				{ id: 'teams_meeting', name: 'Teams Meeting', description: 'Microsoft Teams meeting invitation with join button', category: 'microsoft', brand: 'Teams', icon: '💬' },
				{ id: 'excel_online', name: 'Excel Online', description: 'Excel Online spreadsheet loading with data preview', category: 'microsoft', brand: 'Excel', icon: '📊' },
				{ id: 'dropbox', name: 'Dropbox Transfer', description: 'Dropbox file transfer download page', category: 'cloud', brand: 'Dropbox', icon: '📦' },
				{ id: 'wetransfer', name: 'WeTransfer', description: 'WeTransfer file download page with expiry timer', category: 'cloud', brand: 'WeTransfer', icon: '🔄' },
				{ id: 'voicemail', name: 'Voicemail Message', description: 'Microsoft voicemail notification with audio player', category: 'microsoft', brand: 'Microsoft', icon: '🎤' },
				{ id: 'secure_document', name: 'Secure Document', description: 'Encrypted secure document access with verification', category: 'security', brand: 'Security', icon: '🔒' }
			];
		}
		loadingTemplates = false;
	}

	async function generateBasicAttachment() {
		loading = true;
		result = null;
		try {
			const req = {
				type: format,
				filename: filename || undefined,
				data: { title: subject, body, subject },
				linkUrl,
				htmlContent: format === 'html' ? '' : undefined
			};
			if (format === 'csv') {
				req.data.rows = String(csvRows);
				req.data.columns = csvColumns;
			}
			if (format === 'ics') {
				req.data.summary = icsTitle;
				req.data.location = icsLocation;
				if (icsStartTime) req.data.startTime = icsStartTime;
				if (icsEndTime) req.data.endTime = icsEndTime;
			}
			const res = await api.attachmentGenerator.generate(req);
			if (res && res.data) {
				result = res.data;
				addToast('Attachment generated successfully', 'Success');
			} else {
				addToast(res?.message || 'Failed to generate attachment', 'Error');
			}
		} catch (e) {
			addToast('Failed to generate attachment', 'Error');
		}
		loading = false;
	}

	async function generateTemplateAttachment() {
		if (!selectedTemplate) {
			addToast('Please select a template', 'Error');
			return;
		}
		loading = true;
		result = null;
		try {
			const req = {
				type: 'html_template',
				filename: filename || `${selectedTemplate.replace(/_/g, '-')}.html`,
				template: {
					templateId: selectedTemplate,
					...templateConfig
				}
			};
			const res = await api.attachmentGenerator.generate(req);
			if (res && res.data) {
				result = res.data;
				addToast('Template attachment generated successfully', 'Success');
			} else {
				addToast(res?.message || 'Failed to generate template', 'Error');
			}
		} catch (e) {
			addToast('Failed to generate template attachment', 'Error');
		}
		loading = false;
	}

	async function previewTemplate() {
		if (!selectedTemplate) return;
		loading = true;
		try {
			const req = {
				type: 'html_template',
				filename: 'preview.html',
				template: {
					templateId: selectedTemplate,
					...templateConfig
				}
			};
			const res = await api.attachmentGenerator.generate(req);
			if (res && res.data && res.data.content) {
				previewHTML = atob(res.data.content);
				showPreview = true;
			}
		} catch (e) {
			addToast('Failed to generate preview', 'Error');
		}
		loading = false;
	}

	function downloadResult() {
		if (!result || !result.content) return;
		const blob = new Blob([atob(result.content)], { type: result.contentType || 'application/octet-stream' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = result.filename || 'attachment.' + format;
		a.click();
		URL.revokeObjectURL(url);
	}

	function copyBase64() {
		if (!result || !result.content) return;
		navigator.clipboard.writeText(result.content);
		addToast('Base64 content copied to clipboard', 'Success');
	}

	function getTemplatesByCategory(cat) {
		return templates.filter(t => t.category === cat);
	}

	$: selectedTemplateInfo = templates.find(t => t.id === selectedTemplate);
</script>

<HeadTitle title="Attachment Builder" />
<Headline title="Attachment Builder" subtitle="Generate dynamic phishing attachments with branded templates, embedded tracking links, and randomized content." />

<!-- Tab Navigation -->
<div class="flex gap-1 mb-6 mt-6 bg-gray-100 dark:bg-gray-800 rounded-lg p-1">
	<button
		on:click={() => activeTab = 'basic'}
		class="flex-1 py-2.5 px-4 rounded-md text-sm font-medium transition-all {activeTab === 'basic' ? 'bg-white dark:bg-gray-700 text-gray-900 dark:text-white shadow-sm' : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'}"
	>
		Basic Attachments
	</button>
	<button
		on:click={() => activeTab = 'templates'}
		class="flex-1 py-2.5 px-4 rounded-md text-sm font-medium transition-all {activeTab === 'templates' ? 'bg-white dark:bg-gray-700 text-gray-900 dark:text-white shadow-sm' : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'}"
	>
		HTML Templates
		<span class="ml-1 px-1.5 py-0.5 text-xs rounded-full bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300">{templates.length}</span>
	</button>
</div>

<!-- Basic Attachments Tab -->
{#if activeTab === 'basic'}
<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-6 mb-6 transition-colors duration-200">
	<div class="mb-6">
		<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Attachment Format</label>
		<div class="grid grid-cols-1 md:grid-cols-3 gap-3">
			{#each basicFormats as fmt}
				<button
					on:click={() => format = fmt.value}
					class="p-3 border rounded-lg text-left transition-colors {format === fmt.value ? 'border-cta-blue bg-blue-50 dark:bg-blue-900/20 dark:border-blue-500' : 'border-gray-200 dark:border-gray-600 hover:border-gray-300 dark:hover:border-gray-500'}"
				>
					<p class="font-medium text-sm text-gray-800 dark:text-gray-200"><span class="mr-1">{fmt.icon}</span> {fmt.label}</p>
					<p class="text-xs text-gray-500 dark:text-gray-400 mt-1">{fmt.desc}</p>
				</button>
			{/each}
		</div>
	</div>

	<div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
		<div>
			<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Filename (optional)</label>
			<input type="text" bind:value={filename} placeholder="Auto-generated" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
		</div>
		<div>
			<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Subject / Title</label>
			<input type="text" bind:value={subject} class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
		</div>
		<div>
			<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Link URL</label>
			<input type="url" bind:value={linkUrl} placeholder="https://phishing-page.com" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
		</div>
		<div>
			<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Link Label</label>
			<input type="text" bind:value={linkLabel} class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
		</div>
	</div>
	<div class="mb-4">
		<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Body Content</label>
		<textarea bind:value={body} rows="3" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" placeholder="Email or document body text..."></textarea>
	</div>

	{#if format === 'csv'}
		<div class="border-t border-gray-200 dark:border-gray-700 pt-4 mt-4">
			<h3 class="font-medium text-gray-700 dark:text-gray-200 mb-3">CSV Options</h3>
			<div class="grid grid-cols-2 gap-4">
				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Number of Rows</label>
					<input type="number" bind:value={csvRows} min="1" max="100" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
				</div>
				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Columns (comma-separated)</label>
					<input type="text" bind:value={csvColumns} class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
				</div>
			</div>
		</div>
	{/if}

	{#if format === 'ics'}
		<div class="border-t border-gray-200 dark:border-gray-700 pt-4 mt-4">
			<h3 class="font-medium text-gray-700 dark:text-gray-200 mb-3">Calendar Event Options</h3>
			<div class="grid grid-cols-2 gap-4">
				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Event Title</label>
					<input type="text" bind:value={icsTitle} class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
				</div>
				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Location</label>
					<input type="text" bind:value={icsLocation} class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
				</div>
				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Start Time</label>
					<input type="datetime-local" bind:value={icsStartTime} class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
				</div>
				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">End Time</label>
					<input type="datetime-local" bind:value={icsEndTime} class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
				</div>
			</div>
		</div>
	{/if}

	<div class="mt-6">
		<button on:click={generateBasicAttachment} disabled={loading || !linkUrl} class="bg-cta-blue px-6 py-2 text-white rounded-md font-semibold hover:opacity-80 text-sm transition-all duration-200 disabled:opacity-50">
			{loading ? 'Generating...' : 'Generate Attachment'}
		</button>
	</div>
</div>
{/if}

<!-- HTML Templates Tab -->
{#if activeTab === 'templates'}
<div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
	<!-- Template Selection Panel -->
	<div class="lg:col-span-1">
		<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-4 transition-colors duration-200">
			<h3 class="font-semibold text-gray-800 dark:text-gray-200 mb-4">Select Template</h3>

			{#if loadingTemplates}
				<div class="flex items-center justify-center py-8">
					<div class="animate-spin rounded-full h-6 w-6 border-b-2 border-blue-500"></div>
				</div>
			{:else}
				{#each Object.entries(templateCategories) as [catKey, catInfo]}
					{@const catTemplates = getTemplatesByCategory(catKey)}
					{#if catTemplates.length > 0}
						<div class="mb-4">
							<p class="text-xs font-semibold uppercase tracking-wider mb-2" style="color: {catInfo.color}">{catInfo.label}</p>
							<div class="space-y-1.5">
								{#each catTemplates as tmpl}
									<button
										on:click={() => selectedTemplate = tmpl.id}
										class="w-full p-2.5 rounded-lg text-left transition-all border {selectedTemplate === tmpl.id ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20 ring-1 ring-blue-500' : 'border-transparent hover:bg-gray-50 dark:hover:bg-gray-700/50'}"
									>
										<div class="flex items-center gap-2.5">
											<span class="text-lg flex-shrink-0">{tmpl.icon}</span>
											<div class="min-w-0">
												<p class="text-sm font-medium text-gray-800 dark:text-gray-200 truncate">{tmpl.name}</p>
												<p class="text-xs text-gray-500 dark:text-gray-400 truncate">{tmpl.description}</p>
											</div>
										</div>
									</button>
								{/each}
							</div>
						</div>
					{/if}
				{/each}
			{/if}
		</div>
	</div>

	<!-- Template Configuration Panel -->
	<div class="lg:col-span-2">
		{#if selectedTemplate}
			<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-6 mb-6 transition-colors duration-200">
				<div class="flex items-center justify-between mb-6">
					<div>
						<h3 class="font-semibold text-gray-800 dark:text-gray-200">
							{#if selectedTemplateInfo}
								{selectedTemplateInfo.icon} {selectedTemplateInfo.name}
							{/if}
						</h3>
						{#if selectedTemplateInfo}
							<p class="text-sm text-gray-500 dark:text-gray-400 mt-1">{selectedTemplateInfo.description}</p>
						{/if}
					</div>
					<button on:click={previewTemplate} disabled={loading} class="px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-md hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300 transition-colors">
						Preview
					</button>
				</div>

				<!-- Configuration Form -->
				<div class="space-y-4">
					<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
						<div>
							<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Link URL (Phishing Page)</label>
							<input type="url" bind:value={templateConfig.linkUrl} placeholder="https://your-phishing-page.com" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
						</div>
						<div>
							<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Document Name</label>
							<input type="text" bind:value={templateConfig.documentName} placeholder="Document.pdf" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
						</div>
						<div>
							<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Sender Name</label>
							<input type="text" bind:value={templateConfig.senderName} placeholder="IT Department" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
						</div>
						<div>
							<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Sender Email</label>
							<input type="email" bind:value={templateConfig.senderEmail} placeholder="it@company.com" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
						</div>
						<div>
							<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Company Name</label>
							<input type="text" bind:value={templateConfig.companyName} placeholder="Organization" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
						</div>
						<div>
							<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">File Size</label>
							<input type="text" bind:value={templateConfig.fileSize} placeholder="2.4 MB" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
						</div>
					</div>

					<div>
						<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Custom Message (optional)</label>
						<textarea bind:value={templateConfig.message} rows="2" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" placeholder="Leave empty for default message..."></textarea>
					</div>

					{#if selectedTemplate === 'wetransfer'}
						<div>
							<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Expiry (hours)</label>
							<input type="number" bind:value={templateConfig.expiryHours} min="1" max="720" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
						</div>
					{/if}

					<div>
						<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Output Filename</label>
						<input type="text" bind:value={filename} placeholder="Auto-generated from template" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
					</div>

					<div class="flex items-center gap-3 pt-2">
						<label class="relative inline-flex items-center cursor-pointer">
							<input type="checkbox" bind:checked={templateConfig.antiSandbox} class="sr-only peer" />
							<div class="w-9 h-5 bg-gray-200 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-600 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all dark:after:border-gray-500 peer-checked:bg-blue-600"></div>
						</label>
						<div>
							<span class="text-sm font-medium text-gray-700 dark:text-gray-300">Anti-Sandbox Delay</span>
							<p class="text-xs text-gray-500 dark:text-gray-400">Adds a 3-second delay before showing content (bypasses automated scanners)</p>
						</div>
					</div>
				</div>

				<div class="flex gap-3 mt-6 pt-4 border-t border-gray-200 dark:border-gray-700">
					<button on:click={generateTemplateAttachment} disabled={loading} class="bg-cta-blue px-6 py-2.5 text-white rounded-md font-semibold hover:opacity-80 text-sm transition-all duration-200 disabled:opacity-50 flex-1">
						{loading ? 'Generating...' : 'Generate Template'}
					</button>
					<button on:click={previewTemplate} disabled={loading} class="px-6 py-2.5 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-md font-semibold hover:bg-gray-50 dark:hover:bg-gray-700 text-sm transition-all duration-200 disabled:opacity-50">
						Preview
					</button>
				</div>
			</div>
		{:else}
			<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-12 text-center transition-colors duration-200">
				<div class="text-4xl mb-4">📄</div>
				<h3 class="text-lg font-semibold text-gray-700 dark:text-gray-300 mb-2">Select a Template</h3>
				<p class="text-sm text-gray-500 dark:text-gray-400">Choose a branded HTML template from the panel on the left to configure and generate your attachment.</p>
			</div>
		{/if}
	</div>
</div>
{/if}

<!-- Preview Modal -->
{#if showPreview}
	<!-- svelte-ignore a11y-click-events-have-key-events -->
	<div class="fixed inset-0 bg-black/60 z-50 flex items-center justify-center p-4" on:click|self={() => showPreview = false}>
		<div class="bg-white dark:bg-gray-800 rounded-lg shadow-2xl max-w-3xl w-full max-h-[85vh] flex flex-col">
			<div class="flex items-center justify-between p-4 border-b border-gray-200 dark:border-gray-700">
				<h3 class="font-semibold text-gray-800 dark:text-gray-200">Template Preview</h3>
				<button on:click={() => showPreview = false} class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
					<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
				</button>
			</div>
			<div class="flex-1 overflow-auto p-1">
				<iframe srcdoc={previewHTML} class="w-full h-full min-h-[500px] border-0 rounded" sandbox="allow-same-origin" title="Template Preview"></iframe>
			</div>
		</div>
	</div>
{/if}

<!-- Result -->
{#if result}
	<div class="bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg p-6 mt-6 transition-colors duration-200">
		<h2 class="text-lg font-semibold text-green-800 dark:text-green-300 mb-3">Attachment Generated</h2>
		<div class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-4">
			<div>
				<p class="text-xs text-gray-500 dark:text-gray-400 mb-0.5">Filename</p>
				<p class="text-sm font-medium text-gray-800 dark:text-gray-200">{result.filename}</p>
			</div>
			<div>
				<p class="text-xs text-gray-500 dark:text-gray-400 mb-0.5">Content Type</p>
				<p class="text-sm font-medium text-gray-800 dark:text-gray-200">{result.contentType}</p>
			</div>
			<div>
				<p class="text-xs text-gray-500 dark:text-gray-400 mb-0.5">Size</p>
				<p class="text-sm font-medium text-gray-800 dark:text-gray-200">{(result.size / 1024).toFixed(1)} KB</p>
			</div>
			<div>
				<p class="text-xs text-gray-500 dark:text-gray-400 mb-0.5">Encoding</p>
				<p class="text-sm font-medium text-gray-800 dark:text-gray-200">Base64</p>
			</div>
		</div>
		<div class="flex gap-3">
			<button on:click={downloadResult} class="bg-green-600 px-4 py-2 text-white rounded-md font-semibold hover:bg-green-700 text-sm transition-all duration-200">
				Download {result.filename}
			</button>
			<button on:click={copyBase64} class="px-4 py-2 border border-green-300 dark:border-green-700 text-green-700 dark:text-green-300 rounded-md font-semibold hover:bg-green-100 dark:hover:bg-green-900/30 text-sm transition-all duration-200">
				Copy Base64
			</button>
			{#if result.contentType === 'text/html'}
				<button on:click={() => { previewHTML = atob(result.content); showPreview = true; }} class="px-4 py-2 border border-green-300 dark:border-green-700 text-green-700 dark:text-green-300 rounded-md font-semibold hover:bg-green-100 dark:hover:bg-green-900/30 text-sm transition-all duration-200">
					Preview
				</button>
			{/if}
		</div>
	</div>
{/if}
