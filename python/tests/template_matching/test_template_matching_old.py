"""Pytest Template Matching Module.

Some older tests I decided to leave in.
"""

import unittest
from pathlib import Path

from adb_auto_player.image_manipulation import IO
from adb_auto_player.template_matching import TemplateMatcher

TEST_DATA_DIR = Path(__file__).parent / "data"


class TestTemplateMatching(unittest.TestCase):
    """Pytest Template Matching."""

    def test_similar_image_templates(self) -> None:
        """Test similar_image with templates."""
        f1 = TEST_DATA_DIR / "records_formation_1.png"
        f2 = TEST_DATA_DIR / "records_formation_2.png"
        # f5 = TEST_DATA_DIR / "records_formation_5.png"
        # f6 = TEST_DATA_DIR / "records_formation_6.png"
        # f7 = TEST_DATA_DIR / "records_formation_7.png"

        result = TemplateMatcher.similar_image(
            base_image=IO.load_image(f1),
            template_image=IO.load_image(f1),
        )
        self.assertTrue(result)

        result = TemplateMatcher.similar_image(
            base_image=IO.load_image(f1),
            template_image=IO.load_image(f2),
        )
        self.assertFalse(result)
