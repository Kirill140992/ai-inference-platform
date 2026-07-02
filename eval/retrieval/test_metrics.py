#!/usr/bin/env python3
"""Hand-computed expected values for metrics.py.

Run: python3 eval/retrieval/test_metrics.py
All tests fail with NotImplementedError until metrics.py is implemented —
that's the #3 work loop, not a bug.
"""

import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import metrics  # noqa: E402


class TestRecallAtK(unittest.TestCase):
    def test_hit_inside_k(self):
        self.assertEqual(metrics.recall_at_k({"a"}, ["b", "a", "c"], 2), 1.0)

    def test_hit_outside_k(self):
        self.assertEqual(metrics.recall_at_k({"a"}, ["b", "a", "c"], 1), 0.0)

    def test_no_hit_at_all(self):
        self.assertEqual(metrics.recall_at_k({"z"}, ["b", "a", "c"], 3), 0.0)

    def test_multiple_relevant_partial(self):
        # 1 of 2 relevant docs in top-2 → 0.5
        self.assertEqual(metrics.recall_at_k({"a", "d"}, ["a", "b", "c", "d"], 2), 0.5)


class TestMRR(unittest.TestCase):
    def test_first_position(self):
        self.assertEqual(metrics.mrr({"a"}, ["a", "b", "c"]), 1.0)

    def test_second_position(self):
        self.assertEqual(metrics.mrr({"a"}, ["b", "a", "c"]), 0.5)

    def test_not_retrieved(self):
        self.assertEqual(metrics.mrr({"z"}, ["b", "a", "c"]), 0.0)

    def test_first_relevant_counts(self):
        # both "a" and "c" relevant; first hit is at rank 1
        self.assertEqual(metrics.mrr({"a", "c"}, ["a", "b", "c"]), 1.0)


class TestNDCGAtK(unittest.TestCase):
    def test_perfect_ranking(self):
        self.assertAlmostEqual(metrics.ndcg_at_k({"a"}, ["a", "b", "c"], 3), 1.0, places=6)

    def test_single_relevant_at_rank_3(self):
        # DCG = 1/log2(4) = 0.5; IDCG = 1.0
        self.assertAlmostEqual(metrics.ndcg_at_k({"a"}, ["x", "y", "a"], 5), 0.5, places=6)

    def test_two_relevant_one_displaced(self):
        # relevant={a,b}, ranked=[a,x,b], k=3:
        # DCG  = 1/log2(2) + 1/log2(4)          = 1.5
        # IDCG = 1/log2(2) + 1/log2(3)          ≈ 1.630930
        # nDCG ≈ 1.5 / 1.630930                 ≈ 0.919721
        self.assertAlmostEqual(metrics.ndcg_at_k({"a", "b"}, ["a", "x", "b"], 3), 0.919721, places=5)

    def test_nothing_relevant_retrieved(self):
        self.assertEqual(metrics.ndcg_at_k({"z"}, ["a", "b", "c"], 3), 0.0)


if __name__ == "__main__":
    unittest.main()
