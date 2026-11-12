# Feature ideas brief: parked ideas

*   **Status:** Parked. Nothing here is designed or approved to build.
*   **Date:** 2026-11-11
*   **Origin:** one idea from the owner, plus a market research and brainstorm round.
*   **Sources:** the pages below were opened by research agents on 2026-11-11 and were not opened a
    second time. Reddit, SalonGeek and Capterra blocked every attempt and were not read. Two vendor
    numbers found in the first round (a deposits figure by Kitomba and a rebooking figure by
    Phorest) have no published sample or method and are left out.

A fourth idea from the same round, booking the next appointment when a visit is done, is not
parked: it gets its own brief and design.

--------------------------------------------------------------------------------

## 1. Deposits and no-shows ("Anticipo y faltas")

Mark a deposit on an appointment: the amount, and whether she saw it in her bank. Show a list of
appointments that still lack their deposit, and a no-show count per client.

**Evidence: strong.**

*   Costa Rica practice: of 8 studio pages opened, 4 require a deposit, 2 name SINPE and 3 ask for
    proof of payment. Example, a solo nail studio in San José asking ₡5.000 by SINPE:
    <https://inklaws.carrd.co/>
*   App Store reviews by people who run a studio ask for it, in English and in Spanish. Acuity
    (US, 2024-06-03): "I need to track so many deposits from different clients". Fresha (ES,
    2024-09-09): "Me encantaría poder cobrar adelantado solo a clientas con inasistencias".
*   A review of 105 healthcare studies (abstract only) names a prior no-show as a top predictor of
    the next one, which supports the count per client:
    <https://econpapers.repec.org/RePEc:eee:hepoli:v:122:y:2018:i:4:p:412-421>
*   No study that tests deposits themselves was found.

**Notes.**

*   The app takes no payments. The deposit is a mark she sets by hand.
*   False SINPE receipts are a known fraud in Costa Rica, so the mark should mean "seen in the
    bank", not "receipt received" (a suggestion, not from a source).
*   `appointments` already has the `no_show` status and `paid_cents`
    (`internal/db/migrations/00001_init.sql`).

## 2. Monthly total ("Total del mes")

One figure: the money collected this month, split by cash and transfer, minus expenses she types in
quickly. A list of done visits that are not fully paid is a small part of it.

**Evidence: strong to medium for the total; weak to medium for the unpaid list.**

*   INEC ENAMEH 2025: 69.0% of Costa Rica household micro-businesses keep no accounting of any
    kind (read from a mirror copy of the same file):
    <https://admin.inec.cr/sites/default/files/2026-01/reenameh2025.pdf>
*   McKenzie and Woodruff, surveys of over 20,000 small firms in 7 countries: record keeping has
    the strongest link to profit of the four practice groups. It is a correlation:
    <https://www.nber.org/system/files/working_papers/w21505/w21505.pdf>
*   Drexler, Fischer and Schoar, a randomised trial in the Dominican Republic with beauty salons
    among the businesses: simple rule-of-thumb bookkeeping helped, standard accounting training
    did not: <https://web.mit.edu/aschoar/www/KIS-DFS-March2013.pdf>
*   Unpaid visits: a video by a nail tech about clients who ask for credit has many first-person
    stories in its comments, but the answer most of them give is to refuse credit, not to track it:
    <https://www.youtube.com/watch?v=7LKJ2aa4fJE>

**Notes.**

*   `paid_cents` exists; the payment method does not.
*   `docs/prd.md` says "No incluye inventario, nómina ni contabilidad", and reports are a release 1
    non-goal. A design for this idea changes those lines in the same change, and keeps the feature
    to one figure and one list.

## 3. Inventory ("Inventario")

The owner's idea, and the app's user liked it. She records what she bought of each product and how
much each service uses. Products are linked to services. The app estimates when each product runs
out and gives a purchase list early, with a line in the daily push.

**Evidence: weak in the sources. It stays on the list because the real user asked for it.**

*   McKenzie and Woodruff (same paper as above): stock control shows no link to profit.
*   Of 692 Spanish App Store reviews of salon apps, 8 mention inventory, and 7 of those are about
    apps for salons with staff. No first-person account of running out of product was found.
*   Better supported: the product cost of each service. Videos by nail techs about it have 60,000
    to 85,000 views.
*   One support for stock itself: a Costa Rica manicurist had to throw away more than 150 expired
    gels (La Nación, 2025-10-21):
    <https://www.nacion.com/el-pais/tuve-que-botar-mas-de-150-esmaltes-asi-viven-las/P3NKYS7WS5HSNHZFHHYG56KRGY/story/>

**Notes.**

*   Vendor material says solo owners stop logging each use. Keep the entry small: a few key
    products, "one unit lasts N services" instead of units of measure, and a "count" action that
    resets the stock to what she really has.
*   Store only events (products, links to services, purchases, counts). Stock and the run-out date
    are computed on read, like the due list.
*   Add the product cost and margin beside each service price: this part has the better evidence.
*   `docs/prd.md` says "No incluye inventario". A design for this idea changes that line in the
    same change.
