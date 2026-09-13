-- Année du modèle, saisie par le chauffeur dans le formulaire véhicule (0 = inconnue).
-- Colonne cosmétique : affichage dans les listes et le détail (VehiclePreview.Year).
ALTER TABLE vehicles ADD COLUMN IF NOT EXISTS year SMALLINT NOT NULL DEFAULT 0;
