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
	let format = 'pdf';
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
	let qrSize = 300;
	let invoiceCompany = 'Acme Corporation';
	let invoiceNumber = '';
	let invoiceAmount = '';
	let invoiceDescription = 'Professional Services - Monthly Subscription';
	let invoiceRecipient = 'Valued Customer';

	// Evasion options
	let enableAntiSandbox = false;
	let enableURLObfuscation = false;
	let obfuscationType = 'base64'; // 'base64', 'hex', 'double_encode'
	let enableMetadataStrip = true;
	let enableRandomPadding = false;
	let enableTimeBomb = false;
	let timeBombHours = 48;

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
		{ value: 'pdf', label: 'PDF Document', desc: 'Professional PDF with embedded phishing link and customizable content', icon: '📄', category: 'Documents', evasion: 'high' },
		{ value: 'invoice_pdf', label: 'Invoice PDF', desc: 'Realistic invoice/receipt PDF with payment link and line items', icon: '🧾', category: 'Documents', evasion: 'high' },
		{ value: 'qr_code', label: 'QR Code (PNG)', desc: 'QR code image encoding the phishing URL — bypasses URL filters', icon: '📱', category: 'Evasion', evasion: 'critical' },
		{ value: 'ics', label: 'Calendar Invite (ICS)', desc: 'Calendar event with embedded link — auto-opens in Outlook/Gmail', icon: '📅', category: 'Social Engineering', evasion: 'high' },
		{ value: 'eml', label: 'Email File (EML)', desc: 'Forwarded email file — appears as a legitimate forwarded message', icon: '📧', category: 'Social Engineering', evasion: 'medium' },
		{ value: 'html', label: 'HTML Document', desc: 'Standalone HTML page with embedded link and optional JS redirect', icon: '🌐', category: 'Web', evasion: 'medium' },
		{ value: 'svg', label: 'SVG Image', desc: 'SVG with embedded clickable link — often bypasses attachment filters', icon: '🖼️', category: 'Evasion', evasion: 'critical' },
		{ value: 'csv', label: 'CSV Spreadsheet', desc: 'CSV with formula injection or data lure containing link', icon: '📊', category: 'Data', evasion: 'low' }
	];

	const templateCategories = {
		microsoft: { label: 'Microsoft', color: '#0078d4' },
		document: { label: 'Documents', color: '#d13438' },
		google: { label: 'Google', color: '#4285f4' },
		cloud: { label: 'Cloud Storage', color: '#0061ff' },
		security: { label: 'Security', color: '#667eea' },
		collaboration: { label: 'Collaboration', color: '#611f69' },
		social: { label: 'Social Media', color: '#e1306c' },
		finance: { label: 'Finance', color: '#00457c' },
		shipping: { label: 'Shipping', color: '#d40511' },
		entertainment: { label: 'Entertainment', color: '#e50914' },
		communication: { label: 'Communication', color: '#25d366' },
		business: { label: 'Business', color: '#00a1e0' }
	};

	const evasionColors = {
		critical: { bg: 'bg-red-100 dark:bg-red-900/30', text: 'text-red-700 dark:text-red-300', label: 'Critical' },
		high: { bg: 'bg-orange-100 dark:bg-orange-900/30', text: 'text-orange-700 dark:text-orange-300', label: 'High' },
		medium: { bg: 'bg-yellow-100 dark:bg-yellow-900/30', text: 'text-yellow-700 dark:text-yellow-300', label: 'Medium' },
		low: { bg: 'bg-gray-100 dark:bg-gray-800', text: 'text-gray-600 dark:text-gray-400', label: 'Low' }
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

	function obfuscateURL(url) {
		if (!enableURLObfuscation) return url;
		switch (obfuscationType) {
			case 'base64':
				return `data:text/html;base64,${btoa(`<script>location='${url}'<\/script>`)}`;
			case 'hex':
				return url.split('').map(c => '%' + c.charCodeAt(0).toString(16).padStart(2, '0')).join('');
			case 'double_encode':
				return encodeURIComponent(encodeURIComponent(url));
			default:
				return url;
		}
	}

	async function generateBasicAttachment() {
		loading = true;
		result = null;
		try {
			const processedUrl = enableURLObfuscation ? obfuscateURL(linkUrl) : linkUrl;
			const req = {
				type: format,
				filename: filename || undefined,
				data: { title: subject, body, subject, linkLabel },
				linkUrl: processedUrl,
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
			if (format === 'qr_code') {
				req.data.size = String(qrSize);
			}
			if (format === 'invoice_pdf') {
				req.data.companyName = invoiceCompany;
				if (invoiceNumber) req.data.invoiceNumber = invoiceNumber;
				if (invoiceAmount) req.data.amount = invoiceAmount;
				req.data.description = invoiceDescription;
				req.data.recipientName = invoiceRecipient;
			}
			if (enableAntiSandbox) {
				req.data.antiSandbox = 'true';
			}
			if (enableRandomPadding) {
				req.data.randomPadding = 'true';
			}
			if (enableMetadataStrip) {
				req.data.stripMetadata = 'true';
			}
			if (enableTimeBomb) {
				req.data.timeBombHours = String(timeBombHours);
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
		const raw = atob(result.content);
		const bytes = new Uint8Array(raw.length);
		for (let i = 0; i < raw.length; i++) bytes[i] = raw.charCodeAt(i);
		const blob = new Blob([bytes], { type: result.contentType || 'application/octet-stream' });
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
	$: selectedFormatInfo = basicFormats.find(f => f.value === format);
</script>

<HeadTitle title="Attachment Builder" />
<Headline title="Attachment Builder" subtitle="Generate dynamic phishing attachments with branded templates, embedded tracking links, evasion techniques, and randomized content." />

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
<div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
	<!-- Left: Format Selection -->
	<div class="lg:col-span-1">
		<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-4 transition-colors duration-200">
			<h3 class="font-semibold text-gray-800 dark:text-gray-200 mb-4">Select Format</h3>
			<div class="space-y-2">
				{#each basicFormats as fmt}
					<button
						on:click={() => format = fmt.value}
						class="w-full p-3 rounded-lg text-left transition-all border {format === fmt.value ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20 ring-1 ring-blue-500' : 'border-transparent hover:bg-gray-50 dark:hover:bg-gray-700/50'}"
					>
						<div class="flex items-start gap-2.5">
							<span class="text-lg flex-shrink-0 mt-0.5">{fmt.icon}</span>
							<div class="min-w-0 flex-1">
								<div class="flex items-center gap-2">
									<p class="text-sm font-medium text-gray-800 dark:text-gray-200">{fmt.label}</p>
									<span class="px-1.5 py-0.5 text-[10px] font-semibold rounded {evasionColors[fmt.evasion].bg} {evasionColors[fmt.evasion].text}">{evasionColors[fmt.evasion].label}</span>
								</div>
								<p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{fmt.desc}</p>
							</div>
						</div>
					</button>
				{/each}
			</div>
		</div>
	</div>

	<!-- Right: Configuration -->
	<div class="lg:col-span-2 space-y-6">
		<!-- Format Configuration -->
		<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-6 transition-colors duration-200">
			<div class="flex items-center justify-between mb-5">
				<div>
					<h3 class="font-semibold text-gray-800 dark:text-gray-200">
						{#if selectedFormatInfo}{selectedFormatInfo.icon} {selectedFormatInfo.label}{/if}
					</h3>
					<p class="text-sm text-gray-500 dark:text-gray-400 mt-0.5">{#if selectedFormatInfo}{selectedFormatInfo.desc}{/if}</p>
				</div>
				{#if selectedFormatInfo}
					<span class="px-2 py-1 text-xs font-semibold rounded {evasionColors[selectedFormatInfo.evasion].bg} {evasionColors[selectedFormatInfo.evasion].text}">
						Evasion: {evasionColors[selectedFormatInfo.evasion].label}
					</span>
				{/if}
			</div>

			<!-- Common Fields -->
			<div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Link URL (Phishing Page)</label>
					<input type="url" bind:value={linkUrl} placeholder="https://your-phishing-page.com" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
				</div>
				<div>
					<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Filename (optional)</label>
					<input type="text" bind:value={filename} placeholder="Auto-generated" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
				</div>
			</div>

			<!-- Format-specific: PDF / EML / HTML -->
			{#if format === 'pdf' || format === 'eml' || format === 'html'}
				<div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
					<div>
						<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Subject / Title</label>
						<input type="text" bind:value={subject} class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
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
			{/if}

			<!-- Format-specific: QR Code -->
			{#if format === 'qr_code'}
				<div class="border-t border-gray-200 dark:border-gray-700 pt-4 mt-2">
					<h4 class="font-medium text-gray-700 dark:text-gray-200 mb-3 text-sm">QR Code Options</h4>
					<div class="grid grid-cols-2 gap-4">
						<div>
							<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Image Size (px)</label>
							<input type="number" bind:value={qrSize} min="100" max="1000" step="50" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
						</div>
					</div>
					<div class="mt-3 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg">
						<p class="text-xs text-red-700 dark:text-red-300"><strong>High Evasion:</strong> QR codes bypass most email URL scanners and link protection services. The URL is encoded as an image, making it invisible to text-based filters. Effective against Microsoft Defender, Proofpoint, and Mimecast URL rewriting.</p>
					</div>
				</div>
			{/if}

			<!-- Format-specific: Invoice PDF -->
			{#if format === 'invoice_pdf'}
				<div class="border-t border-gray-200 dark:border-gray-700 pt-4 mt-2">
					<h4 class="font-medium text-gray-700 dark:text-gray-200 mb-3 text-sm">Invoice Details</h4>
					<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
						<div>
							<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Company Name</label>
							<input type="text" bind:value={invoiceCompany} class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
						</div>
						<div>
							<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Invoice Number (auto if empty)</label>
							<input type="text" bind:value={invoiceNumber} placeholder="Auto-generated" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
						</div>
						<div>
							<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Amount (auto if empty)</label>
							<input type="text" bind:value={invoiceAmount} placeholder="$1,234.56" class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
						</div>
						<div>
							<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Recipient Name</label>
							<input type="text" bind:value={invoiceRecipient} class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
						</div>
					</div>
					<div class="mt-3">
						<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Description</label>
						<input type="text" bind:value={invoiceDescription} class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
					</div>
				</div>
			{/if}

			<!-- Format-specific: CSV -->
			{#if format === 'csv'}
				<div class="border-t border-gray-200 dark:border-gray-700 pt-4 mt-2">
					<h4 class="font-medium text-gray-700 dark:text-gray-200 mb-3 text-sm">CSV Options</h4>
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

			<!-- Format-specific: ICS -->
			{#if format === 'ics'}
				<div class="border-t border-gray-200 dark:border-gray-700 pt-4 mt-2">
					<h4 class="font-medium text-gray-700 dark:text-gray-200 mb-3 text-sm">Calendar Event Options</h4>
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
					<div class="mt-3 p-3 bg-orange-50 dark:bg-orange-900/20 border border-orange-200 dark:border-orange-800 rounded-lg">
						<p class="text-xs text-orange-700 dark:text-orange-300"><strong>Social Engineering Tip:</strong> ICS files auto-open in Outlook and Gmail calendar. Use a meeting title like "Urgent: Password Reset Required" with a link in the description for maximum engagement.</p>
					</div>
				</div>
			{/if}

			<!-- Format-specific: SVG -->
			{#if format === 'svg'}
				<div class="border-t border-gray-200 dark:border-gray-700 pt-4 mt-2">
					<div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-3">
						<div>
							<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Subject / Title</label>
							<input type="text" bind:value={subject} class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
						</div>
						<div>
							<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Link Label</label>
							<input type="text" bind:value={linkLabel} class="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm" />
						</div>
					</div>
					<div class="p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg">
						<p class="text-xs text-red-700 dark:text-red-300"><strong>Critical Evasion:</strong> SVG files can contain embedded JavaScript and clickable links. Most email gateways don't scan SVG content, making this format highly effective at bypassing URL analysis and sandbox detonation.</p>
					</div>
				</div>
			{/if}

			<!-- Generate Button -->
			<div class="mt-6 pt-4 border-t border-gray-200 dark:border-gray-700">
				<button on:click={generateBasicAttachment} disabled={loading || !linkUrl} class="bg-cta-blue px-6 py-2.5 text-white rounded-md font-semibold hover:opacity-80 text-sm transition-all duration-200 disabled:opacity-50 w-full sm:w-auto">
					{loading ? 'Generating...' : 'Generate Attachment'}
				</button>
			</div>
		</div>

		<!-- Evasion Options Panel -->
		<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-6 transition-colors duration-200">
			<h3 class="font-semibold text-gray-800 dark:text-gray-200 mb-1">Evasion Options</h3>
			<p class="text-xs text-gray-500 dark:text-gray-400 mb-4">Configure anti-detection and obfuscation techniques for the generated attachment.</p>

			<div class="space-y-4">
				<!-- Anti-Sandbox Delay -->
				<div class="flex items-start gap-3">
					<label class="relative inline-flex items-center cursor-pointer mt-0.5">
						<input type="checkbox" bind:checked={enableAntiSandbox} class="sr-only peer" />
						<div class="w-9 h-5 bg-gray-200 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-600 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all dark:after:border-gray-500 peer-checked:bg-blue-600"></div>
					</label>
					<div>
						<span class="text-sm font-medium text-gray-700 dark:text-gray-300">Anti-Sandbox Delay</span>
						<p class="text-xs text-gray-500 dark:text-gray-400">Adds a 3-5 second delay before rendering content. Bypasses automated sandbox detonation that has short timeouts.</p>
					</div>
				</div>

				<!-- URL Obfuscation -->
				<div class="flex items-start gap-3">
					<label class="relative inline-flex items-center cursor-pointer mt-0.5">
						<input type="checkbox" bind:checked={enableURLObfuscation} class="sr-only peer" />
						<div class="w-9 h-5 bg-gray-200 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-600 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all dark:after:border-gray-500 peer-checked:bg-blue-600"></div>
					</label>
					<div class="flex-1">
						<span class="text-sm font-medium text-gray-700 dark:text-gray-300">URL Obfuscation</span>
						<p class="text-xs text-gray-500 dark:text-gray-400">Encodes the phishing URL to bypass static URL pattern matching in email gateways.</p>
						{#if enableURLObfuscation}
							<div class="flex gap-2 mt-2">
								{#each [{ v: 'base64', l: 'Base64 Redirect' }, { v: 'hex', l: 'Hex Encode' }, { v: 'double_encode', l: 'Double URL Encode' }] as opt}
									<button
										on:click={() => obfuscationType = opt.v}
										class="px-2.5 py-1 text-xs rounded-md border transition-colors {obfuscationType === opt.v ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300' : 'border-gray-300 dark:border-gray-600 text-gray-600 dark:text-gray-400 hover:border-gray-400'}"
									>{opt.l}</button>
								{/each}
							</div>
						{/if}
					</div>
				</div>

				<!-- Metadata Strip -->
				<div class="flex items-start gap-3">
					<label class="relative inline-flex items-center cursor-pointer mt-0.5">
						<input type="checkbox" bind:checked={enableMetadataStrip} class="sr-only peer" />
						<div class="w-9 h-5 bg-gray-200 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-600 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all dark:after:border-gray-500 peer-checked:bg-blue-600"></div>
					</label>
					<div>
						<span class="text-sm font-medium text-gray-700 dark:text-gray-300">Strip Metadata</span>
						<p class="text-xs text-gray-500 dark:text-gray-400">Removes identifying metadata (author, creation tool, timestamps) from generated files to avoid attribution.</p>
					</div>
				</div>

				<!-- Random Padding -->
				<div class="flex items-start gap-3">
					<label class="relative inline-flex items-center cursor-pointer mt-0.5">
						<input type="checkbox" bind:checked={enableRandomPadding} class="sr-only peer" />
						<div class="w-9 h-5 bg-gray-200 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-600 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all dark:after:border-gray-500 peer-checked:bg-blue-600"></div>
					</label>
					<div>
						<span class="text-sm font-medium text-gray-700 dark:text-gray-300">Random Padding</span>
						<p class="text-xs text-gray-500 dark:text-gray-400">Adds random invisible content to change file hash on each generation. Defeats hash-based signature detection.</p>
					</div>
				</div>

				<!-- Time Bomb -->
				<div class="flex items-start gap-3">
					<label class="relative inline-flex items-center cursor-pointer mt-0.5">
						<input type="checkbox" bind:checked={enableTimeBomb} class="sr-only peer" />
						<div class="w-9 h-5 bg-gray-200 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-blue-300 dark:peer-focus:ring-blue-800 rounded-full peer dark:bg-gray-600 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all dark:after:border-gray-500 peer-checked:bg-blue-600"></div>
					</label>
					<div class="flex-1">
						<span class="text-sm font-medium text-gray-700 dark:text-gray-300">Time Bomb</span>
						<p class="text-xs text-gray-500 dark:text-gray-400">Embeds a JavaScript timer that disables the phishing link after the specified hours. Limits forensic analysis window.</p>
						{#if enableTimeBomb}
							<div class="mt-2">
								<input type="number" bind:value={timeBombHours} min="1" max="720" class="w-24 rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white text-xs" />
								<span class="text-xs text-gray-500 dark:text-gray-400 ml-1">hours</span>
							</div>
						{/if}
					</div>
				</div>
			</div>
		</div>
	</div>
</div>
{/if}

<!-- HTML Templates Tab -->
{#if activeTab === 'templates'}
<div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
	<!-- Template Selection Panel -->
	<div class="lg:col-span-1">
		<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-4 transition-colors duration-200 max-h-[80vh] overflow-y-auto">
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
