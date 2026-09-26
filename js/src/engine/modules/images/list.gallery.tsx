import React, { useState } from "react";
import { ModuleViewProps } from "@engine/controllers/registry";
import { imageUrl, ImageRef } from "./controllers/images";
import { useImageFolders, useFolderImages } from "./controllers/folders";
import FolderDeck from "./controllers/FolderDeck";
import useT from "@engine/controllers/useT";

const masonryStyle: React.CSSProperties = {
    margin: "-1.5rem",
    padding: "1rem",
    columns: "220px",
    columnGap: 12,
};

// Named LIST view "gallery": full-bleed masonry wall (CSS columns), folders as deck-of-cards tiles above it.
const ImagesGallery: React.FC<ModuleViewProps> = ({ data, navigate, module }) => {
    const t = useT();
    const [openFolder, setOpenFolder] = useState<string | null>(null);
    const { folders } = useImageFolders(module);
    const { images: folderImages } = useFolderImages(module, openFolder);

    const renderTile = (row: any) => {
        const ref: ImageRef = {
            id: row.id,
            hash: row.preview || row.hash,
            filename: row.filename,
            mime_type: row.mime_type,
        };
        const name = row.filename || `#${row.id}`;
        return (
            <img
                key={row.id}
                src={imageUrl(ref)}
                alt={name}
                title={name}
                loading="lazy"
                onClick={() => navigate(`/${module}/${row.id}`)}
                style={{
                    width: "100%",
                    display: "block",
                    marginBottom: 12,
                    borderRadius: 8,
                    breakInside: "avoid",
                    cursor: "pointer",
                }}
            />
        );
    };

    if (openFolder) {
        return (
            <div style={{ padding: "1rem" }}>
                <div className="d-flex align-items-center mb-3">
                    <button className="btn btn-outline-secondary btn-sm me-2" onClick={() => setOpenFolder(null)}>
                        {t("Back to folders")}
                    </button>
                    <span className="fw-semibold">{openFolder}</span>
                </div>
                {folderImages.length === 0 ? (
                    <div className="text-muted">{t("No images.")}</div>
                ) : (
                    <div style={masonryStyle}>{folderImages.map(renderTile)}</div>
                )}
            </div>
        );
    }

    const rows = Array.isArray(data) ? data : [];
    const standalone = rows.filter((row: any) => !row.folder);

    if (folders.length === 0 && standalone.length === 0) {
        return <div className="text-muted p-3">{t("No images.")}</div>;
    }

    return (
        <div>
            {folders.length > 0 && (
                <div
                    style={{
                        display: "grid",
                        gridTemplateColumns: "repeat(auto-fill, minmax(160px, 1fr))",
                        gap: 16,
                        padding: "1rem",
                    }}
                >
                    {folders.map((f) => (
                        <FolderDeck key={f.folder} folder={f} onClick={() => setOpenFolder(f.folder)} />
                    ))}
                </div>
            )}
            {standalone.length > 0 && <div style={masonryStyle}>{standalone.map(renderTile)}</div>}
        </div>
    );
};

export default ImagesGallery;
