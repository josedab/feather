import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'Feather',
  tagline: 'High-Performance Real-Time Feature Store for Machine Learning',
  favicon: 'img/favicon.ico',

  future: {
    v4: true,
  },

  url: 'https://feather-store.github.io',
  baseUrl: '/',

  organizationName: 'feather-store',
  projectName: 'feather',

  onBrokenLinks: 'throw',
  onBrokenMarkdownLinks: 'warn',

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  markdown: {
    mermaid: true,
  },

  themes: [
    '@docusaurus/theme-mermaid',
    [
      '@easyops-cn/docusaurus-search-local',
      {
        hashed: true,
        language: ['en'],
        highlightSearchTermsOnTargetPage: true,
        explicitSearchResultPath: true,
      },
    ],
  ],

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.ts',
          editUrl: 'https://github.com/feather-store/feather/tree/main/website/',
          showLastUpdateTime: true,
          showLastUpdateAuthor: true,
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    image: 'img/feather-social-card.png',
    colorMode: {
      defaultMode: 'light',
      disableSwitch: false,
      respectPrefersColorScheme: true,
    },
    announcementBar: {
      id: 'star_us',
      content:
        '⭐ If you like Feather, give it a star on <a target="_blank" rel="noopener noreferrer" href="https://github.com/feather-store/feather">GitHub</a>!',
      backgroundColor: '#4CAF50',
      textColor: '#ffffff',
      isCloseable: true,
    },
    navbar: {
      title: 'Feather',
      logo: {
        alt: 'Feather Logo',
        src: 'img/logo.svg',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'docsSidebar',
          position: 'left',
          label: 'Docs',
        },
        {
          to: '/docs/api-reference',
          label: 'API',
          position: 'left',
        },
        {
          to: '/docs/adr/',
          label: 'ADRs',
          position: 'left',
        },
        {
          href: 'https://github.com/feather-store/feather',
          position: 'right',
          className: 'header-github-link',
          'aria-label': 'GitHub repository',
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Documentation',
          items: [
            {
              label: 'Getting Started',
              to: '/docs/getting-started',
            },
            {
              label: 'Core Concepts',
              to: '/docs/concepts/architecture',
            },
            {
              label: 'API Reference',
              to: '/docs/api-reference',
            },
          ],
        },
        {
          title: 'Guides',
          items: [
            {
              label: 'Deployment',
              to: '/docs/guides/deployment',
            },
            {
              label: 'Observability',
              to: '/docs/guides/observability',
            },
            {
              label: 'Performance',
              to: '/docs/guides/performance',
            },
          ],
        },
        {
          title: 'Community',
          items: [
            {
              label: 'GitHub Discussions',
              href: 'https://github.com/feather-store/feather/discussions',
            },
            {
              label: 'Contributing',
              to: '/docs/contributing',
            },
            {
              label: 'Code of Conduct',
              href: 'https://github.com/feather-store/feather/blob/main/CODE_OF_CONDUCT.md',
            },
          ],
        },
        {
          title: 'More',
          items: [
            {
              label: 'GitHub',
              href: 'https://github.com/feather-store/feather',
            },
            {
              label: 'Releases',
              href: 'https://github.com/feather-store/feather/releases',
            },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} Feather Contributors. Built with Docusaurus.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
      additionalLanguages: ['bash', 'go', 'python', 'yaml', 'json', 'protobuf', 'rust', 'java', 'typescript'],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
