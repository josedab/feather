import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docsSidebar: [
    'intro',
    'getting-started',
    {
      type: 'category',
      label: 'Core Concepts',
      collapsed: false,
      items: [
        'concepts/architecture',
        'concepts/tiered-storage',
        'concepts/feature-groups',
        'concepts/point-in-time',
      ],
    },
    {
      type: 'category',
      label: 'Guides',
      items: [
        'guides/deployment',
        'guides/observability',
        'guides/performance',
        'guides/vector-search',
        'guides/drift-detection',
        'guides/offline-sync',
        'guides/freshness',
      ],
    },
    {
      type: 'category',
      label: 'SDKs',
      items: [
        'sdks/go',
        'sdks/python',
        'sdks/rest-api',
      ],
    },
    'api-reference',
    'configuration',
    {
      type: 'category',
      label: 'Architecture Decisions',
      collapsed: true,
      items: [
        'adr/index',
        'adr/tiered-storage',
        'adr/go-implementation-language',
        'adr/point-in-time-versioned-keys',
      ],
    },
    'comparison',
    'troubleshooting',
    'contributing',
  ],
};

export default sidebars;
