"""Feather dbt CLI.

Command-line interface for syncing dbt models to Feather's feature catalog.

Usage:
    feather-dbt sync [OPTIONS]
    feather-dbt validate [OPTIONS]
    feather-dbt status [OPTIONS]
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Optional

from feather_dbt.adapter import FeatherDBTAdapter, SyncOptions


def create_parser() -> argparse.ArgumentParser:
    """Create the argument parser."""
    parser = argparse.ArgumentParser(
        prog="feather-dbt",
        description="Sync dbt models to Feather feature catalog",
    )

    parser.add_argument(
        "--server",
        "-s",
        default="http://localhost:8080",
        help="Feather server URL (default: http://localhost:8080)",
    )
    parser.add_argument(
        "--api-key",
        help="API key for authentication",
    )
    parser.add_argument(
        "--output",
        "-o",
        choices=["json", "text"],
        default="text",
        help="Output format (default: text)",
    )

    subparsers = parser.add_subparsers(dest="command", help="Available commands")

    # Sync command
    sync_parser = subparsers.add_parser("sync", help="Sync dbt models to Feather")
    sync_parser.add_argument(
        "--project-dir",
        "-p",
        help="Path to dbt project directory",
    )
    sync_parser.add_argument(
        "--manifest",
        "-m",
        help="Path to manifest.json file",
    )
    sync_parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Validate without persisting changes",
    )
    sync_parser.add_argument(
        "--tags",
        nargs="+",
        help="Filter models by tag",
    )
    sync_parser.add_argument(
        "--models",
        nargs="+",
        help="Filter by model name patterns",
    )
    sync_parser.add_argument(
        "--include-sources",
        action="store_true",
        help="Include dbt sources",
    )
    sync_parser.add_argument(
        "--include-metrics",
        action="store_true",
        default=True,
        help="Include dbt metrics (default: True)",
    )
    sync_parser.add_argument(
        "--owner",
        help="Owner to assign to all features",
    )
    sync_parser.add_argument(
        "--team",
        help="Team to assign to all features",
    )
    sync_parser.add_argument(
        "--default-entity-type",
        default="unknown",
        help="Default entity type (default: unknown)",
    )

    # Validate command
    validate_parser = subparsers.add_parser(
        "validate", help="Validate dbt manifest without syncing"
    )
    validate_parser.add_argument(
        "--project-dir",
        "-p",
        help="Path to dbt project directory",
    )
    validate_parser.add_argument(
        "--manifest",
        "-m",
        help="Path to manifest.json file",
    )

    # Status command
    subparsers.add_parser("status", help="Get sync status")

    return parser


def run_sync(
    args: argparse.Namespace,
    adapter: FeatherDBTAdapter,
) -> int:
    """Run the sync command."""
    options = SyncOptions(
        dry_run=args.dry_run,
        tags=args.tags,
        models=args.models,
        include_sources=args.include_sources,
        include_metrics=args.include_metrics,
        owner=args.owner,
        team=args.team,
        default_entity_type=args.default_entity_type,
    )

    try:
        result = adapter.sync(options=options)
    except FileNotFoundError as e:
        print(f"Error: {e}", file=sys.stderr)
        return 1
    except Exception as e:
        print(f"Sync failed: {e}", file=sys.stderr)
        return 1

    if args.output == "json":
        print(
            json.dumps(
                {
                    "success": result.success,
                    "features_created": result.features_created,
                    "features_updated": result.features_updated,
                    "features_skipped": result.features_skipped,
                    "errors": [
                        {"model": e.model_name, "column": e.column, "message": e.message}
                        for e in result.errors
                    ],
                    "project_name": result.project_name,
                },
                indent=2,
            )
        )
    else:
        print(f"Project: {result.project_name}")
        print(f"Manifest version: {result.manifest_version}")
        print()
        print(f"Features created: {result.features_created}")
        print(f"Features updated: {result.features_updated}")
        print(f"Features skipped: {result.features_skipped}")

        if result.errors:
            print(f"\nErrors ({len(result.errors)}):")
            for error in result.errors:
                col_info = f".{error.column}" if error.column else ""
                print(f"  - {error.model_name}{col_info}: {error.message}")

        if result.success:
            print("\nSync completed successfully!")
        else:
            print("\nSync completed with errors.")

    return 0 if result.success else 1


def run_validate(
    args: argparse.Namespace,
    adapter: FeatherDBTAdapter,
) -> int:
    """Run the validate command."""
    try:
        result = adapter.validate()
    except FileNotFoundError as e:
        print(f"Error: {e}", file=sys.stderr)
        return 1
    except Exception as e:
        print(f"Validation failed: {e}", file=sys.stderr)
        return 1

    if args.output == "json":
        print(json.dumps(result, indent=2))
    else:
        is_valid = result.get("valid", False)
        feature_count = result.get("features", 0)
        errors = result.get("errors", [])
        project = result.get("project_name", "unknown")

        print(f"Project: {project}")
        print(f"Features found: {feature_count}")
        print(f"Valid: {'Yes' if is_valid else 'No'}")

        if errors:
            print(f"\nErrors ({len(errors)}):")
            for error in errors:
                print(f"  - {error.get('model_name', 'unknown')}: {error.get('message', '')}")

    return 0 if result.get("valid", False) else 1


def run_status(
    args: argparse.Namespace,
    adapter: FeatherDBTAdapter,
) -> int:
    """Run the status command."""
    try:
        result = adapter.status()
    except Exception as e:
        print(f"Failed to get status: {e}", file=sys.stderr)
        return 1

    if args.output == "json":
        print(json.dumps(result, indent=2))
    else:
        last_sync = result.get("last_sync_at")
        if last_sync:
            print(f"Last sync: {last_sync}")
            print(f"Success: {result.get('last_sync_success', 'unknown')}")
            print(f"Project: {result.get('project_name', 'unknown')}")
            print()
            print(f"Features created: {result.get('features_created', 0)}")
            print(f"Features updated: {result.get('features_updated', 0)}")
            print(f"Features skipped: {result.get('features_skipped', 0)}")
            print(f"Errors: {result.get('error_count', 0)}")
        else:
            print("No sync has been performed yet.")

    return 0


def main(argv: Optional[list] = None) -> int:
    """Main entry point."""
    parser = create_parser()
    args = parser.parse_args(argv)

    if not args.command:
        parser.print_help()
        return 0

    # Determine manifest path
    project_dir = getattr(args, "project_dir", None)
    manifest_path = getattr(args, "manifest", None)

    # Default to current directory if nothing specified
    if project_dir is None and manifest_path is None:
        project_dir = "."

    adapter = FeatherDBTAdapter(
        feather_url=args.server,
        project_dir=project_dir,
        manifest_path=manifest_path,
        api_key=args.api_key,
    )

    try:
        if args.command == "sync":
            return run_sync(args, adapter)
        elif args.command == "validate":
            return run_validate(args, adapter)
        elif args.command == "status":
            return run_status(args, adapter)
        else:
            parser.print_help()
            return 0
    finally:
        adapter.close()


if __name__ == "__main__":
    sys.exit(main())
