CREATE UNIQUE INDEX idx_bookings_unique_active_passenger_trip
    ON bookings (passenger_id, trip_id)
    WHERE status IN ('created', 'paymentPending', 'pendingApproval', 'approved', 'inProgress');
