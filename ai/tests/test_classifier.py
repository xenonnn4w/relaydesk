from app.main import classify, ClassifyRequest


def test_incident_safety_rail_marks_production_outage_urgent():
    result = classify(ClassifyRequest(text="Production webhooks are down with 502 errors"))
    assert result.category == "incident"
    assert result.priority == "urgent"
    assert result.confidence >= 0.88


def test_billing_request_gets_billing_policy():
    result = classify(ClassifyRequest(text="Our invoice charged the workspace twice"))
    assert result.category == "billing"
    assert result.priority == "high"

