"""Feather dbt integration.

This package provides tools for syncing dbt models to Feather's feature catalog.

Example:
    >>> from feather_dbt import FeatherDBTAdapter
    >>>
    >>> adapter = FeatherDBTAdapter(
    ...     feather_url="http://localhost:8080",
    ...     project_dir="./my_dbt_project"
    ... )
    >>>
    >>> # Sync all models
    >>> result = adapter.sync()
    >>> print(f"Created {result.features_created} features")
    >>>
    >>> # Sync specific models
    >>> result = adapter.sync(models=["user_features", "item_features"])
"""

from feather_dbt.adapter import FeatherDBTAdapter, SyncOptions, SyncResult

__version__ = "1.0.0"
__all__ = ["FeatherDBTAdapter", "SyncOptions", "SyncResult"]
