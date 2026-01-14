/**
 * Feather Feature Catalog UI - Main Application
 */

// API Configuration
const API_BASE = window.location.origin;

// Application State
const state = {
    features: [],
    filteredFeatures: [],
    entityTypes: new Set(),
    categories: new Set(),
    tags: new Set(),
    currentView: 'features',
    selectedFeature: null,
    searchQuery: '',
    filters: {
        entityType: null,
        category: null,
        tags: [],
        status: null,
    },
};

// Initialize the application
document.addEventListener('DOMContentLoaded', async () => {
    // Initialize Mermaid for lineage graphs
    mermaid.initialize({ startOnLoad: false, theme: 'neutral' });

    // Set up routing
    window.addEventListener('hashchange', handleRouteChange);
    handleRouteChange();

    // Set up search
    setupSearch();

    // Load initial data
    await loadFeatures();
});

// Route handler
function handleRouteChange() {
    const hash = window.location.hash || '#/';
    const path = hash.slice(1);

    // Update active nav
    document.querySelectorAll('.sidebar-link').forEach(link => {
        link.classList.remove('active', 'bg-indigo-50', 'text-indigo-700', 'border-l-4', 'border-indigo-700');
    });

    let activeNav = 'features';
    if (path.startsWith('/entities')) activeNav = 'entities';
    else if (path.startsWith('/lineage')) activeNav = 'lineage';
    else if (path.startsWith('/vectors')) activeNav = 'vectors';

    const activeLink = document.querySelector(`[data-nav="${activeNav}"]`);
    if (activeLink) {
        activeLink.classList.add('active', 'bg-indigo-50', 'text-indigo-700', 'border-l-4', 'border-indigo-700');
    }

    // Render appropriate view
    switch (true) {
        case path === '/' || path === '':
            state.currentView = 'features';
            renderFeatureList();
            break;
        case path.startsWith('/feature/'):
            const featureName = decodeURIComponent(path.slice(9));
            showFeatureDetail(featureName);
            break;
        case path === '/entities':
            state.currentView = 'entities';
            renderEntityTypes();
            break;
        case path === '/lineage':
            state.currentView = 'lineage';
            renderLineageView();
            break;
        case path === '/vectors':
            state.currentView = 'vectors';
            renderVectorIndexes();
            break;
        default:
            state.currentView = 'features';
            renderFeatureList();
    }
}

// API Functions
async function fetchAPI(path) {
    try {
        const response = await fetch(`${API_BASE}${path}`);
        if (!response.ok) {
            throw new Error(`API error: ${response.status}`);
        }
        return await response.json();
    } catch (error) {
        console.error('API fetch error:', error);
        return null;
    }
}

async function loadFeatures() {
    const data = await fetchAPI('/v1/catalog/features');
    if (data && data.features) {
        state.features = data.features;

        // Extract unique values for filters
        state.features.forEach(f => {
            if (f.entity_type) state.entityTypes.add(f.entity_type);
            if (f.category) state.categories.add(f.category);
            if (f.tags) f.tags.forEach(t => state.tags.add(t));
        });

        applyFilters();
        renderFilterPanel();
    } else {
        // Show demo data if API not available
        loadDemoData();
    }
}

function loadDemoData() {
    state.features = [
        {
            name: 'user_purchase_count_7d',
            description: 'Number of purchases in the last 7 days',
            data_type: 'int64',
            entity_type: 'user',
            category: 'engagement',
            tags: ['ml', 'real-time'],
            status: 'active',
            owner: 'ml-team',
        },
        {
            name: 'user_avg_order_value',
            description: 'Average order value across all orders',
            data_type: 'float64',
            entity_type: 'user',
            category: 'financial',
            tags: ['ml', 'batch'],
            status: 'active',
            owner: 'data-team',
        },
        {
            name: 'item_embedding',
            description: 'Product embedding vector for recommendations',
            data_type: 'vector',
            entity_type: 'item',
            category: 'embeddings',
            tags: ['ml', 'vector'],
            status: 'active',
            owner: 'ml-team',
        },
        {
            name: 'user_is_premium',
            description: 'Whether user has premium subscription',
            data_type: 'bool',
            entity_type: 'user',
            category: 'profile',
            tags: ['profile'],
            status: 'active',
            owner: 'product-team',
        },
    ];

    state.features.forEach(f => {
        if (f.entity_type) state.entityTypes.add(f.entity_type);
        if (f.category) state.categories.add(f.category);
        if (f.tags) f.tags.forEach(t => state.tags.add(t));
    });

    applyFilters();
    renderFilterPanel();
    renderFeatureList();
}

// Search setup
function setupSearch() {
    const searchInput = document.getElementById('global-search');
    if (searchInput) {
        searchInput.addEventListener('input', (e) => {
            state.searchQuery = e.target.value.toLowerCase();
            applyFilters();
            if (state.currentView === 'features') {
                renderFeatureList();
            }
        });
    }
}

// Filter functions
function applyFilters() {
    state.filteredFeatures = state.features.filter(f => {
        // Search query
        if (state.searchQuery) {
            const searchFields = [f.name, f.description, f.entity_type, ...(f.tags || [])].join(' ').toLowerCase();
            if (!searchFields.includes(state.searchQuery)) return false;
        }

        // Entity type filter
        if (state.filters.entityType && f.entity_type !== state.filters.entityType) return false;

        // Category filter
        if (state.filters.category && f.category !== state.filters.category) return false;

        // Status filter
        if (state.filters.status && f.status !== state.filters.status) return false;

        // Tags filter
        if (state.filters.tags.length > 0) {
            const featureTags = f.tags || [];
            if (!state.filters.tags.some(t => featureTags.includes(t))) return false;
        }

        return true;
    });
}

// Modal functions
function showModal(content) {
    const modal = document.getElementById('feature-modal');
    const modalContent = document.getElementById('modal-content');
    modalContent.innerHTML = content;
    modal.classList.remove('hidden');
}

function closeModal() {
    const modal = document.getElementById('feature-modal');
    modal.classList.add('hidden');
}

// Click outside to close modal
document.addEventListener('click', (e) => {
    const modal = document.getElementById('feature-modal');
    if (e.target === modal) {
        closeModal();
    }
});

// Utility functions
function formatDataType(type) {
    const typeColors = {
        'int64': 'bg-blue-100 text-blue-800',
        'float64': 'bg-green-100 text-green-800',
        'string': 'bg-yellow-100 text-yellow-800',
        'bool': 'bg-purple-100 text-purple-800',
        'vector': 'bg-pink-100 text-pink-800',
        'timestamp': 'bg-orange-100 text-orange-800',
        'bytes': 'bg-gray-100 text-gray-800',
    };
    return typeColors[type] || 'bg-gray-100 text-gray-800';
}

function formatStatus(status) {
    const statusColors = {
        'active': 'bg-green-100 text-green-800',
        'deprecated': 'bg-yellow-100 text-yellow-800',
        'experimental': 'bg-blue-100 text-blue-800',
        'disabled': 'bg-red-100 text-red-800',
    };
    return statusColors[status] || 'bg-gray-100 text-gray-800';
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}
