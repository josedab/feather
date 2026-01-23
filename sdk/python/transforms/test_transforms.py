"""Tests for the Feather Python Transform SDK."""

import unittest

from feather_transforms import (
    TransformRegistry,
    feather_transform,
    normalize_score,
    test_transform,
)


class TestDecoratorRegistration(unittest.TestCase):
    """Test that @feather_transform registers functions correctly."""

    def setUp(self) -> None:
        self.registry = TransformRegistry()

    def test_builtin_normalize_score_registered(self) -> None:
        info = self.registry.get("normalize_score")
        self.assertIsNotNone(info)
        self.assertIn("raw_score", info["input_schema"])
        self.assertIn("normalized_score", info["output_schema"])

    def test_custom_transform_registration(self) -> None:
        @feather_transform(
            name="double_value",
            input_schema={"value": "float"},
            output_schema={"doubled": "float"},
        )
        def double_value(data):
            return {"doubled": data["value"] * 2}

        info = self.registry.get("double_value")
        self.assertIsNotNone(info)
        self.assertEqual(info["input_schema"], {"value": "float"})
        self.assertEqual(info["output_schema"], {"doubled": "float"})

    def test_decorated_function_still_callable(self) -> None:
        @feather_transform(name="add_one", input_schema={"x": "int"}, output_schema={"y": "int"})
        def add_one(data):
            return {"y": data["x"] + 1}

        result = add_one({"x": 5})
        self.assertEqual(result, {"y": 6})


class TestTestTransformHelper(unittest.TestCase):
    """Test the test_transform() local testing helper."""

    def test_normalize_score_mid_range(self) -> None:
        result = test_transform(normalize_score, {
            "raw_score": 50.0,
            "min_score": 0.0,
            "max_score": 100.0,
        })
        self.assertAlmostEqual(result["normalized_score"], 0.5)

    def test_normalize_score_at_min(self) -> None:
        result = test_transform(normalize_score, {
            "raw_score": 0.0,
            "min_score": 0.0,
            "max_score": 100.0,
        })
        self.assertAlmostEqual(result["normalized_score"], 0.0)

    def test_normalize_score_at_max(self) -> None:
        result = test_transform(normalize_score, {
            "raw_score": 100.0,
            "min_score": 0.0,
            "max_score": 100.0,
        })
        self.assertAlmostEqual(result["normalized_score"], 1.0)

    def test_normalize_score_equal_min_max(self) -> None:
        result = test_transform(normalize_score, {
            "raw_score": 5.0,
            "min_score": 5.0,
            "max_score": 5.0,
        })
        self.assertAlmostEqual(result["normalized_score"], 0.0)


class TestRegistryListing(unittest.TestCase):
    """Test TransformRegistry.list_transforms()."""

    def setUp(self) -> None:
        self.registry = TransformRegistry()

    def test_list_contains_builtin(self) -> None:
        transforms = self.registry.list_transforms()
        self.assertIn("normalize_score", transforms)

    def test_list_excludes_func_reference(self) -> None:
        transforms = self.registry.list_transforms()
        for info in transforms.values():
            self.assertNotIn("func", info)

    def test_list_includes_schemas(self) -> None:
        transforms = self.registry.list_transforms()
        entry = transforms["normalize_score"]
        self.assertIn("input_schema", entry)
        self.assertIn("output_schema", entry)


if __name__ == "__main__":
    unittest.main()
