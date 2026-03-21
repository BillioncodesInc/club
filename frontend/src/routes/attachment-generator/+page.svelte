<script>
	import { api } from '$lib/api/apiProxy.js';
	import HeadTitle from '$lib/components/HeadTitle.svelte';
	import Headline from '$lib/components/Headline.svelte';
	import { addToast } from '$lib/store/toast';

	let loading = false;
	let result = null;

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

	const formats = [
		{ value: 'csv', label: 'CSV Spreadsheet', desc: 'Generates a CSV with randomized data rows' },
		{ value: 'ics', label: 'Calendar Invite (ICS)', desc: 'Generates an ICS calendar event with embedded link' },
		{ value: 'eml', label: 'Email File (EML)', desc: 'Generates a forwarded email with embedded link' },
		{ value: 'html', label: 'HTML Document', desc: 'Generates an HTML document with embedded link' },
		{ value: 'svg', label: 'SVG Image', desc: 'Generates an SVG image with embedded link' }
	];

	async function generateAttachment() {
		loading = true;
		result = null;
		try {
			const req = {
				format,
				filename: filename || undefined,
				subject,
				body,
				linkUrl,
				linkLabel,
				options: {}
			};
			if (format === 'csv') {
				req.options.rows = csvRows;
				req.options.columns = csvColumns;
			}
			if (format === 'ics') {
				req.options.title = icsTitle;
				req.options.location = icsLocation;
				req.options.startTime = icsStartTime;
				req.options.endTime = icsEndTime;
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
</script>

<HeadTitle title="Attachment Generator" />
<Headline title="Attachment Generator" subtitle="Generate dynamic phishing attachments with embedded tracking links and randomized content." />

<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md dark:shadow-none border border-gray-200 dark:border-gray-700 p-6 mb-6 mt-6 transition-colors duration-200">
	<!-- Format Selection -->
	<div class="mb-6">
		<label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Attachment Format</label>
		<div class="grid grid-cols-1 md:grid-cols-3 gap-3">
			{#each formats as fmt}
				<button
					on:click={() => format = fmt.value}
					class="p-3 border rounded-lg text-left transition-colors {format === fmt.value ? 'border-cta-blue bg-blue-50 dark:bg-blue-900/20 dark:border-blue-500' : 'border-gray-200 dark:border-gray-600 hover:border-gray-300 dark:hover:border-gray-500'}"
				>
					<p class="font-medium text-sm text-gray-800 dark:text-gray-200">{fmt.label}</p>
					<p class="text-xs text-gray-500 dark:text-gray-400 mt-1">{fmt.desc}</p>
				</button>
			{/each}
		</div>
	</div>

	<!-- Common Fields -->
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

	<!-- Format-specific options -->
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
		<button on:click={generateAttachment} disabled={loading || !linkUrl} class="bg-cta-blue px-6 py-2 text-white rounded-md font-semibold hover:opacity-80 text-sm transition-all duration-200 disabled:opacity-50">
			{loading ? 'Generating...' : 'Generate Attachment'}
		</button>
	</div>
</div>

<!-- Result -->
{#if result}
	<div class="bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg p-6 transition-colors duration-200">
		<h2 class="text-lg font-semibold text-green-800 dark:text-green-300 mb-3">Attachment Generated</h2>
		<div class="space-y-2 mb-4">
			<p class="text-sm text-gray-700 dark:text-gray-300"><strong>Filename:</strong> {result.filename}</p>
			<p class="text-sm text-gray-700 dark:text-gray-300"><strong>Format:</strong> {result.format}</p>
			<p class="text-sm text-gray-700 dark:text-gray-300"><strong>Size:</strong> {result.size} bytes</p>
			<p class="text-sm text-gray-700 dark:text-gray-300"><strong>Content Type:</strong> {result.contentType}</p>
		</div>
		{#if result.preview}
			<div class="mb-4">
				<h3 class="font-medium text-sm text-gray-700 dark:text-gray-300 mb-1">Preview:</h3>
				<pre class="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded p-3 text-xs font-mono overflow-x-auto max-h-48 text-gray-800 dark:text-gray-200">{result.preview}</pre>
			</div>
		{/if}
		<button on:click={downloadResult} class="bg-green-600 px-4 py-2 text-white rounded-md font-semibold hover:bg-green-700 text-sm transition-all duration-200">
			Download {result.filename}
		</button>
	</div>
{/if}
