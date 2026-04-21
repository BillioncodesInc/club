<script>
	import { menu } from '$lib/consts/navigation';
	import { page } from '$app/stores';
	import { scrollBarClassesVertical } from '$lib/utils/scrollbar';
	import { shouldHideMenuItem } from '$lib/utils/common';
	import { AppStateService } from '$lib/service/appState';
	import { onMount } from 'svelte';
	import { beforeNavigate } from '$app/navigation';

	export let isPinned = false;

	let isExpanded = false;

	// expose collapseMenu for parent to collapse menu on unpin
	export function collapseMenu() {
		isExpanded = false;
	}

	$: isExpanded = isPinned ? true : isExpanded;
	let menuElement;
	let menuItemsElement;
	let hasScrollbar = false;
	let instantCollapse = false;
	let context = {
		current: '',
		companyName: ''
	};

	const appState = AppStateService.instance;
	import { createEventDispatcher } from 'svelte';
	import ConditionalDisplay from '../ConditionalDisplay.svelte';
	const dispatch = createEventDispatcher();

	// check if scrollbar is present
	const checkScrollbar = () => {
		if (menuItemsElement) {
			hasScrollbar = menuItemsElement.scrollHeight > menuItemsElement.clientHeight;
		}
	};

	$: if (isPinned && menuItemsElement) {
		checkScrollbar();
	}

	onMount(() => {
		const unsub = appState.subscribe((s) => {
			context = {
				current: s.context.current,
				companyName: s.context.companyName
			};
		});

		// handle click outside to collapse menu
		const handleClickOutside = (event) => {
			if (!isPinned && isExpanded && menuElement && !menuElement.contains(event.target)) {
				isExpanded = false;
			}
		};

		// check scrollbar on resize
		const resizeObserver = new ResizeObserver(() => {
			if (isPinned) {
				checkScrollbar();
			}
		});

		if (menuItemsElement) {
			resizeObserver.observe(menuItemsElement);
		}

		document.addEventListener('click', handleClickOutside);

		return () => {
			unsub();
			resizeObserver.disconnect();
			document.removeEventListener('click', handleClickOutside);
		};
	});

	// handle navigation to collapse menu
	beforeNavigate(() => {
		if (!isPinned && isExpanded) {
			instantCollapse = true;
			isExpanded = false;
			// reset after a brief moment
			setTimeout(() => {
				instantCollapse = false;
			}, 50);
		}
	});

	$: hasCompanySelected =
		context.current === AppStateService.CONTEXT.COMPANY && context.companyName;

	const icons = {
		dashboard: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
        <path stroke-linecap="round" stroke-linejoin="round" d="m2.25 12 8.954-8.955c.44-.439 1.152-.439 1.591 0L21.75 12M4.5 9.75v10.125c0 .621.504 1.125 1.125 1.125H9.75v-4.875c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125V21h4.125c.621 0 1.125-.504 1.125-1.125V9.75M8.25 21h8.25" />
    </svg>`,

		campaigns_overview: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
        <path stroke-linecap="round" stroke-linejoin="round" d="M8 5h6c2 0 4 2 4 4v4c0 3-3 5-5 5 1.5-1.5 1.5-3 1.5-3" />
        <path stroke-linecap="round" stroke-linejoin="round" d="M14.5 15l-2 2" />
        <circle cx="14" cy="5" r="1" fill="currentColor" />
    </svg>`,
		campaign_templates: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
        <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m0 12.75h7.5m-7.5 3H12M10.5 2.25H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z" />
    </svg>`,

		filters: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
  <path stroke-linecap="round" stroke-linejoin="round" d="M12 3c2.755 0 5.455.232 8.083.678.533.09.917.556.917 1.096v1.044a2.25 2.25 0 0 1-.659 1.591l-5.432 5.432a2.25 2.25 0 0 0-.659 1.591v2.927a2.25 2.25 0 0 1-1.244 2.013L9.75 21v-6.568a2.25 2.25 0 0 0-.659-1.591L3.659 7.409A2.25 2.25 0 0 1 3 5.818V4.774c0-.54.384-1.006.917-1.096A48.32 48.32 0 0 1 12 3Z" />
</svg>
`,

		webhooks: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
        <path stroke-linecap="round" stroke-linejoin="round" d="m3.75 13.5 10.5-11.25L12 10.5h8.25L9.75 21.75 12 13.5H3.75Z" />
    </svg>`,

		recipients_overview: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
  <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 6a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0ZM4.501 20.118a7.5 7.5 0 0 1 14.998 0A17.933 17.933 0 0 1 12 21.75c-2.676 0-5.216-.584-7.499-1.632Z" />
</svg>`,

		recipient_groups: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
        <path stroke-linecap="round" stroke-linejoin="round" d="M18 18.72a9.094 9.094 0 0 0 3.741-.479 3 3 0 0 0-4.682-2.72m.94 3.198.001.031c0 .225-.012.447-.037.666A11.944 11.944 0 0 1 12 21c-2.17 0-4.207-.576-5.963-1.584A6.062 6.062 0 0 1 6 18.719m12 0a5.971 5.971 0 0 0-.941-3.197m0 0A5.995 5.995 0 0 0 12 12.75a5.995 5.995 0 0 0-5.058 2.772m0 0a3 3 0 0 0-4.681 2.72 8.986 8.986 0 0 0 3.74.477m.94-3.197a5.971 5.971 0 0 0-.94 3.197M15 6.75a3 3 0 1 1-6 0 3 3 0 0 1 6 0Zm6 3a2.25 2.25 0 1 1-4.5 0 2.25 2.25 0 0 1 4.5 0Zm-13.5 0a2.25 2.25 0 1 1-4.5 0 2.25 2.25 0 0 1 4.5 0Z" />
    </svg>`,

		domains_overview: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
        <path stroke-linecap="round" stroke-linejoin="round" d="M12 21a9.004 9.004 0 0 0 8.716-6.747M12 21a9.004 9.004 0 0 1-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 0 1 7.843 4.582M12 3a8.997 8.997 0 0 0-7.843 4.582m15.686 0A11.953 11.953 0 0 1 12 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0 1 21 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0 1 12 16.5c-3.162 0-6.133-.815-8.716-2.247m0 0A9.015 9.015 0 0 1 3 12c0-1.605.42-3.113 1.157-4.418" />
    </svg>`,

		pages: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
        <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m2.25 0H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z" />
    </svg>`,

		assets: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
        <path stroke-linecap="round" stroke-linejoin="round" d="m2.25 15.75 5.159-5.159a2.25 2.25 0 0 1 3.182 0l5.159 5.159m-1.5-1.5 1.409-1.409a2.25 2.25 0 0 1 3.182 0l2.909 2.909m-18 3.75h16.5a1.5 1.5 0 0 0 1.5-1.5V6a1.5 1.5 0 0 0-1.5-1.5H3.75A1.5 1.5 0 0 0 2.25 6v12a1.5 1.5 0 0 0 1.5 1.5Zm10.5-11.25h.008v.008h-.008V8.25Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Z" />
    </svg>`,

		emails_overview: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
        <path stroke-linecap="round" stroke-linejoin="round" d="M21.75 6.75v10.5a2.25 2.25 0 0 1-2.25 2.25h-15a2.25 2.25 0 0 1-2.25-2.25V6.75m19.5 0A2.25 2.25 0 0 0 19.5 4.5h-15a2.25 2.25 0 0 0-2.25 2.25m19.5 0v.243a2.25 2.25 0 0 1-1.07 1.916l-7.5 4.615a2.25 2.25 0 0 1-2.36 0L3.32 8.91a2.25 2.25 0 0 1-1.07-1.916V6.75" />
    </svg>`,

		attachments: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
        <path stroke-linecap="round" stroke-linejoin="round" d="m18.375 12.739-7.693 7.693a4.5 4.5 0 0 1-6.364-6.364l10.94-10.94A3 3 0 1 1 19.5 7.372L8.552 18.32m.009-.01-.01.01m5.699-9.941-7.81 7.81a1.5 1.5 0 0 0 2.112 2.13" />
    </svg>`,

		smtp_configurations: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
  <path stroke-linecap="round" stroke-linejoin="round" d="M16.5 12a4.5 4.5 0 1 1-9 0 4.5 4.5 0 0 1 9 0Zm0 0c0 1.657 1.007 3 2.25 3S21 13.657 21 12a9 9 0 1 0-2.636 6.364M16.5 12V8.25" />
</svg>
`,

		api_senders: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
  <path stroke-linecap="round" stroke-linejoin="round" d="M6 12 3.269 3.125A59.769 59.769 0 0 1 21.485 12 59.768 59.768 0 0 1 3.27 20.875L5.999 12Zm0 0h7.5" />
</svg>
`,

		oauth_providers: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
  <path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75 11.25 15 15 9.75m-3-7.036A11.959 11.959 0 0 1 3.598 6 11.99 11.99 0 0 0 3 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285Z" />
</svg>
`,

		proxy: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
  <path stroke-linecap="round" stroke-linejoin="round" d="M7.5 21 3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" />
</svg>
`,

		// New feature icons
		telegram: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
  <path stroke-linecap="round" stroke-linejoin="round" d="M6 12 3.269 3.125A59.769 59.769 0 0 1 21.485 12 59.768 59.768 0 0 1 3.27 20.875L5.999 12Zm0 0h7.5" />
</svg>`,

		sms: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
  <path stroke-linecap="round" stroke-linejoin="round" d="M10.5 1.5H8.25A2.25 2.25 0 0 0 6 3.75v16.5a2.25 2.25 0 0 0 2.25 2.25h7.5A2.25 2.25 0 0 0 18 20.25V3.75a2.25 2.25 0 0 0-2.25-2.25H13.5m-3 0V3h3V1.5m-3 0h3m-3 18.75h3" />
</svg>`,

		live_map: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
  <path stroke-linecap="round" stroke-linejoin="round" d="M9 6.75V15m6-6v8.25m.503 3.498 4.875-2.437c.381-.19.622-.58.622-1.006V4.82c0-.836-.88-1.38-1.628-1.006l-3.869 1.934c-.317.159-.69.159-1.006 0L9.503 3.252a1.125 1.125 0 0 0-1.006 0L3.622 5.689C3.24 5.88 3 6.27 3 6.695V19.18c0 .836.88 1.38 1.628 1.006l3.869-1.934c.317-.159.69-.159 1.006 0l4.994 2.497c.317.158.69.158 1.006 0Z" />
</svg>`,

		domain_rotation: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
  <path stroke-linecap="round" stroke-linejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0 3.181 3.183a8.25 8.25 0 0 0 13.803-3.7M4.031 9.865a8.25 8.25 0 0 1 13.803-3.7l3.181 3.182M2.985 19.644l3.181-3.182" />
</svg>`,

		anti_detection: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
  <path stroke-linecap="round" stroke-linejoin="round" d="M3.98 8.223A10.477 10.477 0 0 0 1.934 12C3.226 16.338 7.244 19.5 12 19.5c.993 0 1.953-.138 2.863-.395M6.228 6.228A10.451 10.451 0 0 1 12 4.5c4.756 0 8.773 3.162 10.065 7.498a10.522 10.522 0 0 1-4.293 5.774M6.228 6.228 3 3m3.228 3.228 3.65 3.65m7.894 7.894L21 21m-3.228-3.228-3.65-3.65m0 0a3 3 0 1 0-4.243-4.243m4.242 4.242L9.88 9.88" />
</svg>`,

		email_warming: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
  <path stroke-linecap="round" stroke-linejoin="round" d="M15.362 5.214A8.252 8.252 0 0 1 12 21 8.25 8.25 0 0 1 6.038 7.047 8.287 8.287 0 0 0 9 9.601a8.983 8.983 0 0 1 3.361-6.867 8.21 8.21 0 0 0 3 2.48Z" />
  <path stroke-linecap="round" stroke-linejoin="round" d="M12 18a3.75 3.75 0 0 0 .495-7.468 5.99 5.99 0 0 0-1.925 3.547 5.975 5.975 0 0 1-2.133-1.001A3.75 3.75 0 0 0 12 18Z" />
</svg>`,

		enhanced_headers: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
  <path stroke-linecap="round" stroke-linejoin="round" d="M17.25 6.75 22.5 12l-5.25 5.25m-10.5 0L1.5 12l5.25-5.25m7.5-3-4.5 16.5" />
</svg>`,

		bot_guard: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
  <path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75 11.25 15 15 9.75m-3-7.036A11.959 11.959 0 0 1 3.598 6 11.99 11.99 0 0 0 3 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285Z" />
</svg>`,

		captured_session: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
  <path stroke-linecap="round" stroke-linejoin="round" d="M7.5 7.5h-.75A2.25 2.25 0 0 0 4.5 9.75v7.5a2.25 2.25 0 0 0 2.25 2.25h7.5a2.25 2.25 0 0 0 2.25-2.25v-7.5a2.25 2.25 0 0 0-2.25-2.25h-.75m0-3-3-3m0 0-3 3m3-3v11.25" />
</svg>`,

		content_balancer: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
  <path stroke-linecap="round" stroke-linejoin="round" d="M12 3v17.25m0 0c-1.472 0-2.882.265-4.185.75M12 20.25c1.472 0 2.882.265 4.185.75M18.75 4.97A48.416 48.416 0 0 0 12 4.5c-2.291 0-4.545.16-6.75.47m13.5 0c1.01.143 2.01.317 3 .52m-3-.52 2.62 10.726c.122.499-.106 1.028-.589 1.202a5.988 5.988 0 0 1-2.031.352 5.988 5.988 0 0 1-2.031-.352c-.483-.174-.711-.703-.59-1.202L18.75 4.971Zm-16.5.52c.99-.203 1.99-.377 3-.52m0 0 2.62 10.726c.122.499-.106 1.028-.589 1.202a5.989 5.989 0 0 1-2.031.352 5.989 5.989 0 0 1-2.031-.352c-.483-.174-.711-.703-.59-1.202L5.25 4.971Z" />
</svg>`,

		webserver_rules: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
  <path stroke-linecap="round" stroke-linejoin="round" d="M5.25 14.25h13.5m-13.5 0a3 3 0 0 1-3-3m3 3a3 3 0 1 0 0 6h13.5a3 3 0 1 0 0-6m-16.5-3a3 3 0 0 1 3-3h13.5a3 3 0 0 1 3 3m-19.5 0a4.5 4.5 0 0 1 .9-2.7L5.737 5.1a3.375 3.375 0 0 1 2.7-1.35h7.126c1.062 0 2.062.5 2.7 1.35l2.587 3.45a4.5 4.5 0 0 1 .9 2.7m0 0a3 3 0 0 1-3 3m0 3h.008v.008h-.008v-.008Zm0-6h.008v.008h-.008v-.008Zm-3 6h.008v.008h-.008v-.008Zm0-6h.008v.008h-.008v-.008Z" />
</svg>`,

		dkim: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
  <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 5.25a3 3 0 0 1 3 3m3 0a6 6 0 0 1-7.029 5.912c-.563-.097-1.159.026-1.563.43L10.5 17.25H8.25v2.25H6v2.25H2.25v-2.818c0-.597.237-1.17.659-1.591l6.499-6.499c.404-.404.527-1 .43-1.563A6 6 0 1 1 21.75 8.25Z" />
</svg>`,

		link_manager: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
  <path stroke-linecap="round" stroke-linejoin="round" d="M13.19 8.688a4.5 4.5 0 0 1 1.242 7.244l-4.5 4.5a4.5 4.5 0 0 1-6.364-6.364l1.757-1.757m13.35-.622 1.757-1.757a4.5 4.5 0 0 0-6.364-6.364l-4.5 4.5a4.5 4.5 0 0 0 1.242 7.244" />
</svg>`,

		attachment_generator: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
  <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m3.75 9v6m3-3H9m1.5-12H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z" />
</svg>`,

		turnstile: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
  <path stroke-linecap="round" stroke-linejoin="round" d="M16.5 10.5V6.75a4.5 4.5 0 1 0-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 0 0 2.25-2.25v-6.75a2.25 2.25 0 0 0-2.25-2.25H6.75a2.25 2.25 0 0 0-2.25 2.25v6.75a2.25 2.25 0 0 0 2.25 2.25Z" />
</svg>`,

		proxy_captures: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
  <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m.75 12 3 3m0 0 3-3m-3 3v-6m-1.5-9H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z" />
</svg>`,
		cookie_store: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
  <path stroke-linecap="round" stroke-linejoin="round" d="M12 18v-5.25m0 0a6.01 6.01 0 0 0 1.5-.189m-1.5.189a6.01 6.01 0 0 1-1.5-.189m3.75 7.478a12.06 12.06 0 0 1-4.5 0m3.75 2.383a14.406 14.406 0 0 1-3 0M14.25 18v-.192c0-.983.658-1.823 1.508-2.316a7.5 7.5 0 1 0-7.517 0c.85.493 1.509 1.333 1.509 2.316V18" />
</svg>`,

		open_redirects: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
  <path stroke-linecap="round" stroke-linejoin="round" d="M13.5 6H5.25A2.25 2.25 0 0 0 3 8.25v10.5A2.25 2.25 0 0 0 5.25 21h10.5A2.25 2.25 0 0 0 18 18.75V10.5m-10.5 6L21 3m0 0h-5.25M21 3v5.25" />
</svg>`,

		evasion_rules: `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
  <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m0-10.036A11.959 11.959 0 0 1 3.598 6 11.99 11.99 0 0 0 3 9.75c0 5.592 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.57-.598-3.75h-.152c-3.196 0-6.1-1.25-8.25-3.286Zm0 13.036h.008v.008H12v-.008Z" />
</svg>`
	};

	const getIconForRoute = (route) => {
		const iconMap = {
			'/dashboard/': 'dashboard',
			'/campaign/': 'campaigns_overview',
			'/campaign-template/': 'campaign_templates',
			'/filter/': 'filters',
			'/webhook/': 'webhooks',
			'/recipient/': 'recipients_overview',
			'/recipient/group/': 'recipient_groups',
			'/domain/': 'domains_overview',
			'/page/': 'pages',
			'/proxy/': 'proxy',
			'/asset/': 'assets',
			'/email/': 'emails_overview',
			'/attachment/': 'attachments',
			'/smtp-configuration/': 'smtp_configurations',
			'/api-sender/': 'api_senders',
			'/oauth-provider/': 'oauth_providers',
			'/telegram/': 'telegram',
			'/sms/': 'sms',
			'/live-map/': 'live_map',
			'/domain-rotation/': 'domain_rotation',
			'/anti-detection/': 'anti_detection',
			'/email-warming/': 'email_warming',
			'/enhanced-headers/': 'enhanced_headers',
			'/bot-guard/': 'bot_guard',
			'/captured-session/': 'captured_session',
			'/content-balancer/': 'content_balancer',
			'/webserver-rules/': 'webserver_rules',
			'/dkim/': 'dkim',
			'/link-manager/': 'link_manager',
			'/attachment-generator/': 'attachment_generator',
			'/turnstile/': 'turnstile',
			'/proxy-captures/': 'proxy_captures',
			'/cookie-store/': 'cookie_store',
			'/open-redirects/': 'open_redirects',
			'/evasion-rules/': 'evasion_rules'
		};

		return icons[iconMap[route] || 'dashboard']; // fallback to dashboard if route not found
	};
</script>

<div class="flex">
	<nav
		aria-label="Primary"
		bind:this={menuElement}
		class="hidden lg:flex flex-col fixed top-16 z-10 bg-gradient-to-b from-pc-darkblue to-indigo-400 dark:from-gray-900 dark:to-gray-800 rounded-br-lg overflow-x-hidden min-h-0 max-h-[calc(100vh-4rem)] box-content border-r-[1px] border-pc-darkblue dark:border-highlight-blue/40"
		class:transition-all={!instantCollapse}
		class:w-40={isExpanded}
		class:w-12={!isExpanded}
		class:!top-[100px]={hasCompanySelected}
		class:!max-h-[calc(100vh-100px)]={hasCompanySelected}
	>
		{#if !isPinned}
			<div
				class="bg-highlight-blue/20 dark:bg-gray-800/70 border-b w-full border-blue-700/30 dark:border-highlight-blue/40 transition-colors duration-200 flex items-center justify-between px-1"
			>
				<div class="flex items-center w-full">
					<button
						class="flex items-center justify-center rounded-md hover:bg-blue-600/30 dark:hover:bg-highlight-blue/20 transition-colors group px-3 py-2"
						on:click={() => (isExpanded = !isExpanded)}
						type="button"
					>
						<svg
							class="text-blue-100 dark:text-highlight-blue duration-200 w-4 h-4 transition-colors"
							class:rotate-180={!isExpanded}
							xmlns="http://www.w3.org/2000/svg"
							fill="none"
							viewBox="0 0 24 24"
							stroke-width="1.5"
							stroke="currentColor"
						>
							<path
								stroke-linecap="round"
								stroke-linejoin="round"
								d="m18.75 4.5-7.5 7.5 7.5 7.5m-6-15L5.25 12l7.5 7.5"
							/>
						</svg>
					</button>
					{#if isExpanded}
						<button
							class="flex items-center justify-center rounded-md hover:bg-blue-600/30 dark:hover:bg-highlight-blue/20 transition-colors group px-3 py-2"
							title={isPinned ? 'Unpin menu' : 'Pin menu'}
							on:click={() => dispatch('pinToggle')}
							type="button"
						>
							{#if isPinned}
								<svg
									width="16"
									height="16"
									viewBox="0 0 24 24"
									fill="currentColor"
									xmlns="http://www.w3.org/2000/svg"
									class="w-4 h-4 text-blue-100 dark:text-highlight-blue"
								>
									<path
										d="M16 9V5a1 1 0 0 0-1-1H9a1 1 0 0 0-1 1v4l-4 4v2h6v5a1 1 0 0 0 2 0v-5h6v-2l-4-4z"
									/>
								</svg>
							{:else}
								<svg
									width="16"
									height="16"
									viewBox="0 0 24 24"
									fill="none"
									xmlns="http://www.w3.org/2000/svg"
									class="w-4 h-4 text-blue-100 dark:text-highlight-blue"
								>
									<path
										d="M16 9V5a1 1 0 0 0-1-1H9a1 1 0 0 0-1 1v4l-4 4v2h6v5a1 1 0 0 0 2 0v-5h6v-2l-4-4z"
										stroke="currentColor"
										stroke-width="2"
									/>
								</svg>
							{/if}
						</button>
					{/if}
				</div>
			</div>
		{/if}

		{#if isPinned}
			<button
				class="absolute top-2 z-10 flex items-center justify-center w-6 h-6 rounded-md bg-blue-600/30 dark:bg-gray-800/70 hover:bg-blue-600/50 dark:hover:bg-gray-700 transition-colors"
				class:right-5={hasScrollbar}
				class:right-2={!hasScrollbar}
				title="Unpin menu"
				on:click={() => dispatch('pinToggle')}
				type="button"
			>
				<svg
					width="16"
					height="16"
					viewBox="0 0 24 24"
					fill="none"
					xmlns="http://www.w3.org/2000/svg"
					class="w-4 h-4 text-blue-100 dark:text-highlight-blue"
				>
					<path
						d="M16 9V5a1 1 0 0 0-1-1H9a1 1 0 0 0-1 1v4l-4 4v2h6v5a1 1 0 0 0 2 0v-5h6v-2l-4-4z"
						stroke="currentColor"
						stroke-width="1.2"
						stroke-linecap="round"
						stroke-linejoin="round"
					/>
				</svg>
			</button>
		{/if}

		<!-- Navigation Items -->
		<div
			bind:this={menuItemsElement}
			class="flex flex-col flex-1 overflow-y-auto overflow-x-hidden {scrollBarClassesVertical} [&::-webkit-scrollbar-track]:bg-cta-blue dark:[&::-webkit-scrollbar-track]:bg-gray-800"
			class:py-4={!isPinned}
		>
			{#each menu as link}
				{#if link.type === 'submenu'}
					<div class="py-1 mt-4 first:mt-0">
						{#if isExpanded}
							<div
								class="px-3 py-2 text-xs font-semibold text-blue-100 dark:text-highlight-blue uppercase tracking-wider transition-colors duration-200"
							>
								{link.label}
							</div>
						{/if}

						<div>
							{#each link.items as item, i (i)}
								<ConditionalDisplay show={item.blackbox ? 'blackbox' : 'both'}>
									<a
										class="flex items-center px-3 py-2 text-sm transition-all duration-150 relative group
                                        {(
											item.route === '/dashboard/'
												? $page.url.pathname.startsWith('/dashboard')
												: $page.url.pathname === item.route
										)
											? 'text-white font-medium bg-active-blue dark:bg-active-blue shadow-md'
											: 'text-blue-100 dark:text-gray-200 hover:shadow-md hover:bg-highlight-blue/80 dark:hover:bg-highlight-blue/20 hover:text-white dark:hover:text-gray-100'}"
										class:hidden={shouldHideMenuItem(item.route)}
										draggable="false"
										href={item.route}
										title={item.label}
									>
										<!-- Icon -->
										<div class="flex-shrink-0 text-blue-100 dark:text-highlight-blue">
											{@html getIconForRoute(item.route)}
										</div>

										{#if isExpanded}
											<span class="ml-3 truncate">
												{#if i === 0}
													Overview
												{:else if item.singleLabel}
													{item.singleLabel}
												{:else}
													{item.label}
												{/if}
											</span>
										{/if}

										{#if item.route === '/dashboard/' ? $page.url.pathname.startsWith('/dashboard') : $page.url.pathname === item.route}
											<div
												class="absolute left-0 top-0 bottom-0 w-1 bg-white dark:bg-highlight-blue"
											></div>
										{/if}
									</a>
								</ConditionalDisplay>
							{/each}
						</div>
					</div>
				{:else}
					<a
						class="flex items-center px-3 py-2 text-sm transition-all duration-150 relative group
                            {$page.url.pathname === link.route
							? 'text-white font-medium bg-active-blue dark:bg-active-blue shadow-md'
							: 'text-blue-100 dark:text-gray-200 hover:text-white dark:hover:text-gray-100 hover:bg-highlight-blue/80 dark:hover:bg-highlight-blue/20'}"
						draggable="false"
						href={link.route}
					>
						<!-- Icon -->
						<div class="flex-shrink-0 text-blue-100 dark:text-highlight-blue">
							{@html icons[link.label]}
						</div>

						{#if isExpanded}
							<span class="ml-3 truncate">{link.label}</span>
						{:else}
							<div
								class="absolute left-14 rounded bg-gray-900 dark:bg-gray-800 text-white dark:text-highlight-blue px-2 py-1 ml-6 text-sm
	                                invisible opacity-0 -translate-x-3 group-hover:visible group-hover:opacity-100 group-hover:translate-x-0
	                                transition-all duration-150 whitespace-nowrap z-50 shadow-lg border dark:border-highlight-blue/40"
							>
								{link.label}
							</div>
						{/if}

						{#if $page.url.pathname === link.route}
							<div class="absolute left-0 top-0 bottom-0 w-1 bg-white dark:bg-highlight-blue"></div>
						{/if}
					</a>
				{/if}
			{/each}
		</div>
	</nav>

	<!-- Main Content -->
	<div class="flex-1">
		<slot />
	</div>
</div>
