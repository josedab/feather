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
        'guides/catalog-ui',
        'guides/dbt-integration',
        'guides/langchain',
        'guides/llamaindex',
        'guides/kubernetes-operator',
        'guides/cloud-storage',
        'guides/llm-embeddings',
        'guides/streaming',
        'guides/dashboard',
      ],
    },
    {
      type: 'category',
      label: 'SDKs',
      items: [
        'sdks/go',
        'sdks/python',
        'sdks/java',
        'sdks/rust',
        'sdks/typescript',
        'sdks/rest-api',
      ],
    },
    'cli',
    'api-reference',
    'configuration',
    {
      type: 'category',
      label: 'Architecture Decisions',
      collapsed: true,
      items: [
        'adr/index',
        'adr/tiered-storage',
        'adr/sharded-cache',
        'adr/go-implementation-language',
        'adr/point-in-time-versioned-keys',
        'adr/hnsw-vector-similarity',
        'adr/object-pooling',
        'adr/single-binary',
        'adr/prometheus-metrics',
      ],
    },
    'comparison',
    'faq',
    'troubleshooting',
    'changelog',
    'contributing',
  ],
};

export default sidebars;
