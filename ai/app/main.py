from __future__ import annotations

import re
from dataclasses import dataclass

from fastapi import FastAPI
from pydantic import BaseModel, Field
from sklearn.feature_extraction.text import TfidfVectorizer
from sklearn.linear_model import LogisticRegression
from sklearn.pipeline import Pipeline


TRAINING_DATA = [
    ("production API is down and customers are blocked", "incident"),
    ("webhooks return 500 errors after deploy", "incident"),
    ("outage affecting all users urgent help", "incident"),
    ("messages stopped syncing in production", "incident"),
    ("charged twice on our latest invoice", "billing"),
    ("need a copy of the receipt for payment", "billing"),
    ("subscription amount on invoice is incorrect", "billing"),
    ("how do we update the billing address", "billing"),
    ("SSO user gets permission denied", "access"),
    ("cannot log in with Okta", "access"),
    ("invite link expired for a new team member", "access"),
    ("role permissions prevent dashboard access", "access"),
    ("please add an export to csv option", "feature"),
    ("it would be useful to support dark mode", "feature"),
    ("feature request for custom reports", "feature"),
    ("can you add a new integration", "feature"),
    ("where can I find the API documentation", "question"),
    ("how does ticket assignment work", "question"),
    ("need help configuring notifications", "question"),
    ("what is included in the starter plan", "question"),
]


class ClassifyRequest(BaseModel):
    text: str = Field(min_length=1, max_length=10_000)


class Classification(BaseModel):
    title: str
    summary: str
    category: str
    priority: str
    confidence: float
    tags: list[str]


@dataclass(frozen=True)
class CategoryPolicy:
    priority: str
    tag: str


POLICIES = {
    "incident": CategoryPolicy("urgent", "incident"),
    "billing": CategoryPolicy("high", "billing"),
    "access": CategoryPolicy("high", "access"),
    "feature": CategoryPolicy("low", "feature-request"),
    "question": CategoryPolicy("normal", "how-to"),
}


def build_model() -> Pipeline:
    texts, labels = zip(*TRAINING_DATA, strict=True)
    model = Pipeline(
        [
            ("tfidf", TfidfVectorizer(ngram_range=(1, 2), stop_words="english")),
            ("classifier", LogisticRegression(max_iter=500, random_state=7)),
        ]
    )
    model.fit(texts, labels)
    return model


MODEL = build_model()
app = FastAPI(title="RelayDesk AI", version="0.1.0")


def clean(text: str) -> str:
    return re.sub(r"\s+", " ", text).strip()


def clipped(text: str, limit: int) -> str:
    normalized = clean(text)
    if len(normalized) <= limit:
        return normalized
    return normalized[: limit - 1].rstrip(" ,.;:-") + "…"


def title_for(text: str) -> str:
    first_sentence = re.split(r"[.!?]\s+", clean(text), maxsplit=1)[0]
    title = clipped(first_sentence, 68)
    return title[0].upper() + title[1:] if title else "New support request"


@app.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok", "model": "tfidf-logistic-regression"}


@app.post("/classify", response_model=Classification)
def classify(request: ClassifyRequest) -> Classification:
    text = clean(request.text)
    probabilities = MODEL.predict_proba([text])[0]
    classes = MODEL.classes_
    best_index = int(probabilities.argmax())
    category = str(classes[best_index])
    confidence = float(probabilities[best_index])

    # Business-critical incident words act as a safety rail around the small demo model.
    incident_terms = ("down", "outage", "production", "blocked", "urgent", "500", "502")
    if any(term in text.lower() for term in incident_terms):
        category = "incident"
        confidence = max(confidence, 0.88)

    policy = POLICIES[category]
    return Classification(
        title=title_for(text),
        summary=clipped(text, 160),
        category=category,
        priority=policy.priority,
        confidence=round(confidence, 2),
        tags=[policy.tag, "ml-triaged"],
    )

