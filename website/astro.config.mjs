// @ts-check
import starlight from '@astrojs/starlight';
import { defineConfig } from 'astro/config';

// https://astro.build/config
export default defineConfig({
	site: "https://infracollect.github.io",
	base: "/infracollect",
	integrations: [
		starlight({
			title: 'infracollect',
			logo: {
				src: './src/assets/infracollect.svg',
			},
			components: {
				SiteTitle: './src/components/SiteTitle.astro',
			},
			social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/infracollect/infracollect' }],
			sidebar: [
				{
					label: "Getting started",
					items: [
						{ label: 'Installation', slug: 'guides/install' },
						{ label: 'Your first job', slug: 'guides/your-first-job' },
						{ label: 'What\'s next', slug: 'guides/whats-next' },
					],
				},
				{
					label: 'Recipes',
					items: [
						{ label: 'AWS', slug: 'recipes/aws' },
					],
				},
				{
					label: 'Reference',
					autogenerate: { directory: 'reference' },
				},
			],
		}),
	],
});
