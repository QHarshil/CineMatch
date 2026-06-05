"""Hard cost ceiling for the CineMatch GCP project.

Cloud Run has no native spend cap: past the free tier, usage just bills at
standard rates, and budget alerts only notify. This Cloud Function is the
backstop. A Cloud Billing budget publishes to a Pub/Sub topic; this function
reads each notification and, once spend passes the budget, disables billing on
the project, which stops every billable service.

Disabling billing affects the whole project, so run CineMatch in its own
dedicated GCP project. Re-enabling billing is a manual step (by design: the site
goes offline rather than charging you).
"""

import base64
import json
import os

import functions_framework
from googleapiclient import discovery

PROJECT_ID = os.environ["GCP_PROJECT_ID"]
PROJECT_NAME = f"projects/{PROJECT_ID}"


@functions_framework.cloud_event
def stop_billing(cloud_event):
    """Disable project billing when actual spend exceeds the budget amount."""
    message = cloud_event.data["message"]
    budget = json.loads(base64.b64decode(message["data"]).decode("utf-8"))

    cost = budget.get("costAmount", 0)
    cap = budget.get("budgetAmount", 0)
    if cost <= cap:
        print(f"cost {cost} within budget {cap}; no action")
        return

    billing = discovery.build("cloudbilling", "v1", cache_discovery=False)
    info = billing.projects().getBillingInfo(name=PROJECT_NAME).execute()
    if not info.get("billingEnabled", False):
        print("billing already disabled")
        return

    # An empty billingAccountName detaches the billing account from the project.
    billing.projects().updateBillingInfo(
        name=PROJECT_NAME, body={"billingAccountName": ""}
    ).execute()
    print(f"billing DISABLED for {PROJECT_NAME} (cost {cost} > budget {cap})")
