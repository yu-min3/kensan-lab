#!/usr/bin/env python3
"""CLI script for running the profile summarizer batch job."""

import argparse
import asyncio
import logging
import sys
from datetime import datetime, timedelta
from zoneinfo import ZoneInfo

from kensan_ai.batch.profile_summarizer import ProfileSummarizer

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
)
logger = logging.getLogger(__name__)


async def run_summarizer(days: int, since: str | None = None) -> int:
    """Run the profile summarizer batch job.

    Args:
        days: Number of days to look back for new facts
        since: Alternative ISO datetime string to use instead of days

    Returns:
        Number of users processed
    """
    summarizer = ProfileSummarizer()

    if since:
        # Parse the since datetime
        try:
            updated_since = datetime.fromisoformat(since)
            if updated_since.tzinfo is None:
                updated_since = updated_since.replace(tzinfo=ZoneInfo("UTC"))
        except ValueError as e:
            logger.error(f"Invalid datetime format: {since}")
            raise SystemExit(1) from e
    else:
        updated_since = datetime.now(ZoneInfo("UTC")) - timedelta(days=days)

    return await summarizer.run_batch(updated_since=updated_since)


def main() -> None:
    """Main entry point for the CLI."""
    parser = argparse.ArgumentParser(
        description="Run profile summarizer batch job to aggregate user facts into profile summaries.",
    )
    parser.add_argument(
        "--days",
        type=int,
        default=1,
        help="Number of days to look back for new facts (default: 1)",
    )
    parser.add_argument(
        "--since",
        type=str,
        default=None,
        help="ISO datetime to start from (e.g., 2024-01-01T00:00:00). Overrides --days.",
    )
    parser.add_argument(
        "--verbose",
        "-v",
        action="store_true",
        help="Enable verbose logging",
    )

    args = parser.parse_args()

    if args.verbose:
        logging.getLogger().setLevel(logging.DEBUG)

    logger.info("Starting profile summarizer batch job")
    logger.info(f"Parameters: days={args.days}, since={args.since}")

    try:
        processed = asyncio.run(run_summarizer(days=args.days, since=args.since))
        logger.info(f"Batch job complete: processed {processed} users")
        sys.exit(0)
    except KeyboardInterrupt:
        logger.info("Batch job interrupted by user")
        sys.exit(130)
    except Exception as e:
        logger.error(f"Batch job failed: {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()
