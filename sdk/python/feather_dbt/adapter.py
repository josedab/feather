"""Feather dbt Adapter.

This module provides an adapter for syncing dbt models to Feather's feature catalog.
"""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from datetime import datetime
from pathlib import Path
from typing import Any, Dict, List, Optional

import httpx


@dataclass
class SyncOptions:
    """Options for dbt sync operation.

    Attributes:
        dry_run: If True, validate but don't persist changes
        tags: Filter models by tag (empty means all)
        models: Filter by model name patterns
        include_sources: Include dbt sources in sync
        include_metrics: Include dbt metrics as features
        entity_type_mapping: Map dbt model tags to entity types
        default_entity_type: Default entity type when no mapping matches
        owner: Owner to assign to all features
        team: Team to assign to all features
    """

    dry_run: bool = False
    tags: Optional[List[str]] = None
    models: Optional[List[str]] = None
    include_sources: bool = False
    include_metrics: bool = True
    entity_type_mapping: Optional[Dict[str, str]] = None
    default_entity_type: str = "unknown"
    owner: Optional[str] = None
    team: Optional[str] = None

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for API request."""
        return {
            "dry_run": self.dry_run,
            "tags": self.tags,
            "models": self.models,
            "include_sources": self.include_sources,
            "include_metrics": self.include_metrics,
            "entity_type_mapping": self.entity_type_mapping,
            "default_entity_type": self.default_entity_type,
            "owner": self.owner,
            "team": self.team,
        }


@dataclass
class SyncError:
    """Error that occurred during sync."""

    model_name: str
    column: Optional[str] = None
    message: str = ""


@dataclass
class FeatureDefinition:
    """Feature definition synced from dbt."""

    name: str
    description: str
    data_type: str
    entity_type: str
    owner: Optional[str] = None
    team: Optional[str] = None
    tags: List[str] = field(default_factory=list)
    category: Optional[str] = None
    status: str = "active"
    version: int = 1
    metadata: Dict[str, str] = field(default_factory=dict)


@dataclass
class SyncResult:
    """Result of a dbt sync operation."""

    success: bool
    features_created: int
    features_updated: int
    features_skipped: int
    errors: List[SyncError]
    features: List[FeatureDefinition]
    synced_at: datetime
    manifest_version: str
    project_name: str


class FeatherDBTAdapter:
    """Adapter for syncing dbt models to Feather's feature catalog.

    This adapter reads dbt manifest.json files and syncs the model/column
    definitions to Feather's feature catalog, enabling automatic feature
    discovery and documentation.

    Args:
        feather_url: URL of the Feather server
        project_dir: Path to dbt project directory (optional)
        manifest_path: Direct path to manifest.json (optional)
        api_key: API key for authentication
        timeout: Request timeout in seconds

    Example:
        >>> adapter = FeatherDBTAdapter(
        ...     feather_url="http://localhost:8080",
        ...     project_dir="./my_dbt_project"
        ... )
        >>> result = adapter.sync()
        >>> print(f"Synced {result.features_created} features")
    """

    def __init__(
        self,
        feather_url: str = "http://localhost:8080",
        project_dir: Optional[str] = None,
        manifest_path: Optional[str] = None,
        api_key: Optional[str] = None,
        timeout: float = 30.0,
    ) -> None:
        self.feather_url = feather_url.rstrip("/")
        self.project_dir = Path(project_dir) if project_dir else None
        self.manifest_path = Path(manifest_path) if manifest_path else None
        self.api_key = api_key
        self.timeout = timeout
        self._client = httpx.Client(timeout=timeout)

    def _get_manifest_path(self) -> Path:
        """Get the path to manifest.json."""
        if self.manifest_path:
            return self.manifest_path

        if self.project_dir:
            # Look for manifest in target directory
            target_manifest = self.project_dir / "target" / "manifest.json"
            if target_manifest.exists():
                return target_manifest

            # Also check current directory
            local_manifest = self.project_dir / "manifest.json"
            if local_manifest.exists():
                return local_manifest

        raise FileNotFoundError(
            "Could not find manifest.json. "
            "Run 'dbt compile' or 'dbt build' first, or specify manifest_path."
        )

    def _load_manifest(self) -> Dict[str, Any]:
        """Load and parse manifest.json."""
        path = self._get_manifest_path()
        with open(path) as f:
            return json.load(f)

    def _make_request(
        self,
        method: str,
        path: str,
        json_data: Optional[Dict[str, Any]] = None,
    ) -> httpx.Response:
        """Make HTTP request to Feather server."""
        url = f"{self.feather_url}{path}"
        headers = {}
        if self.api_key:
            headers["Authorization"] = f"Bearer {self.api_key}"

        response = self._client.request(
            method=method,
            url=url,
            json=json_data,
            headers=headers,
        )
        response.raise_for_status()
        return response

    def sync(
        self,
        options: Optional[SyncOptions] = None,
        manifest: Optional[Dict[str, Any]] = None,
    ) -> SyncResult:
        """Sync dbt models to Feather feature catalog.

        Args:
            options: Sync options (uses defaults if not provided)
            manifest: Pre-loaded manifest dict (loads from file if not provided)

        Returns:
            SyncResult with details of the sync operation

        Raises:
            httpx.HTTPStatusError: If the API request fails
            FileNotFoundError: If manifest.json cannot be found
        """
        if manifest is None:
            manifest = self._load_manifest()

        if options is None:
            options = SyncOptions()

        request_data = {
            "manifest": manifest,
            "options": options.to_dict(),
        }

        response = self._make_request("POST", "/v1/dbt/sync", request_data)
        data = response.json()

        return self._parse_sync_result(data)

    def validate(
        self,
        options: Optional[SyncOptions] = None,
        manifest: Optional[Dict[str, Any]] = None,
    ) -> Dict[str, Any]:
        """Validate dbt manifest without syncing.

        Args:
            options: Sync options
            manifest: Pre-loaded manifest dict

        Returns:
            Validation result with errors and feature count
        """
        if manifest is None:
            manifest = self._load_manifest()

        if options is None:
            options = SyncOptions()

        request_data = {
            "manifest": manifest,
            "options": options.to_dict(),
        }

        response = self._make_request("POST", "/v1/dbt/validate", request_data)
        return response.json()

    def status(self) -> Dict[str, Any]:
        """Get the status of the last dbt sync.

        Returns:
            Status information including last sync time and results
        """
        response = self._make_request("GET", "/v1/dbt/status")
        return response.json()

    def _parse_sync_result(self, data: Dict[str, Any]) -> SyncResult:
        """Parse sync result from API response."""
        errors = [
            SyncError(
                model_name=e.get("model_name", ""),
                column=e.get("column"),
                message=e.get("message", ""),
            )
            for e in data.get("errors", [])
        ]

        features = [
            FeatureDefinition(
                name=f.get("name", ""),
                description=f.get("description", ""),
                data_type=f.get("data_type", "string"),
                entity_type=f.get("entity_type", "unknown"),
                owner=f.get("owner"),
                team=f.get("team"),
                tags=f.get("tags", []),
                category=f.get("category"),
                status=f.get("status", "active"),
                version=f.get("version", 1),
                metadata=f.get("metadata", {}),
            )
            for f in data.get("features", [])
        ]

        synced_at_str = data.get("synced_at", "")
        try:
            synced_at = datetime.fromisoformat(synced_at_str.replace("Z", "+00:00"))
        except (ValueError, AttributeError):
            synced_at = datetime.now()

        return SyncResult(
            success=data.get("success", False),
            features_created=data.get("features_created", 0),
            features_updated=data.get("features_updated", 0),
            features_skipped=data.get("features_skipped", 0),
            errors=errors,
            features=features,
            synced_at=synced_at,
            manifest_version=data.get("manifest_version", ""),
            project_name=data.get("project_name", ""),
        )

    def close(self) -> None:
        """Close the HTTP client."""
        self._client.close()

    def __enter__(self) -> "FeatherDBTAdapter":
        return self

    def __exit__(self, *args: Any) -> None:
        self.close()
