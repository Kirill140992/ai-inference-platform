"""Retrieval quality metrics — recall@k, MRR, nDCG@k.

THE FORMULAS ARE INTENTIONALLY NOT IMPLEMENTED. Implementing them is the
learning core of #3 (docs/book-issues/03-offline-eval.md) — read first,
then code:

- AI Engineering (Chip Huyen), evaluation chapters — why offline eval sets
  and ranking metrics, how they compose into a regression harness.
- Introduction to Information Retrieval (Manning et al.), ch. 8
  "Evaluation in information retrieval" — the canonical definitions of
  MRR and (n)DCG used below.

Work loop: implement one function, run
    python3 eval/retrieval/test_metrics.py
until all tests pass. The tests encode hand-computed expected values, so
they double as worked examples.

All functions use binary relevance and doc-level ids: `relevant` is the
set of correct document ids for one query, `ranked` is the retriever's
output, best first. Each returns a float in [0, 1].
"""

import math  # you'll want math.log2 for nDCG


def recall_at_k(relevant: set, ranked: list, k: int) -> float:
    """Fraction of relevant docs that appear in the top-k of `ranked`.

    Definition: |relevant ∩ ranked[:k]| / |relevant|.
    With a single relevant doc this is binary: 1.0 if it made the top-k.

    Example: relevant={"a"}, ranked=["b","a","c"] → recall@1 = 0.0,
    recall@2 = 1.0.
    """
    raise NotImplementedError("TODO(#3): implement recall@k")


def mrr(relevant: set, ranked: list) -> float:
    """Reciprocal rank of the FIRST relevant doc in `ranked`.

    Definition: 1 / rank of the first relevant hit (1-indexed);
    0.0 if no relevant doc was retrieved at all.

    Example: relevant={"a"}, ranked=["b","a","c"] → 1/2 = 0.5.
    (The dataset-level MRR is the mean of this over all queries —
    the harness does the averaging, this function is per-query.)
    """
    raise NotImplementedError("TODO(#3): implement reciprocal rank")


def ndcg_at_k(relevant: set, ranked: list, k: int) -> float:
    """Normalized Discounted Cumulative Gain at k, binary relevance.

    DCG@k  = sum over positions i=1..k of  rel_i / log2(i + 1),
             where rel_i is 1 if ranked[i-1] is relevant else 0.
    IDCG@k = DCG@k of the ideal ranking (all relevant docs first).
    nDCG@k = DCG@k / IDCG@k   (0.0 if IDCG is 0).

    Example: relevant={"a"}, ranked=["x","y","a"], k=5 →
    DCG = 1/log2(4) = 0.5, IDCG = 1/log2(2) = 1.0, nDCG = 0.5.
    """
    raise NotImplementedError("TODO(#3): implement nDCG@k")
