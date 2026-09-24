import React, { useState } from "react";
import { ModuleViewProps } from "@engine/controllers/registry";
import { imageUrl, ImageRef } from "./controllers/images";
import { useImageFolders, useFolderImages } from "./controllers/folders";
import FolderDeck from "./controllers/FolderDeck";
import useT from "@engine/useT";

const fmtDate = (v: any): string => {
    if (!v) return "";
    const d = new Date(v);
    if (isNaN(d.getTime())) return String(v);
    return `${d.toLocaleDateString()} ${d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}`;
};

const gridStyle: React.CSSProperties = {
    display: "grid",
    gridTemplateColumns: "repeat(auto-fill, minmax(160px, 1fr))",
    gap: 16,
};

// Custom LIST view: image card grid, with folders shown as deck-of-cards tiles above the standalone images.
const ImagesList: React.FC<ModuleViewProps> = ({ data, navigate, module }) => {
    const t = useT();
    const [openFolder, setOpenFolder] = useState<string | null>(null);
    const { folders } = useImageFolders(module);
    const { images: folderImages } = useFolderImages(module, openFolder);

    const renderCard = (row: any) => {
        const ref: ImageRef = {
            id: row.id,
            uuid: row.uuid,
            filename: row.filename,
            mime_type: row.mime_type,
        };
        const name = row.filename || `#${row.id}`;
        return (
            <div
                key={row.id}
                className="card h-100"
                style={{ cursor: "pointer" }}
                onClick={() => navigate(`/${module}/${row.id}`)}
                title={name}
            >
                <div style={{ aspectRatio: "1 / 1", overflow: "hidden", background: "#f5f5f5" }}>
                    <img
                        src={imageUrl(ref)}
                        alt={name}
                        loading="lazy"
                        style={{ width: "100%", height: "100%", objectFit: "cover" }}
                    />
                </div>
                <div className="card-body p-2">
                    <div className="text-truncate small fw-semibold">{name}</div>
                    <div className="text-muted" style={{ fontSize: 12 }}>
                        {fmtDate(row.created)}
                    </div>
                </div>
            </div>
        );
    };

    if (openFolder) {
        return (
            <div>
                <div className="d-flex align-items-center mb-3">
                    <button className="btn btn-outline-secondary btn-sm me-2" onClick={() => setOpenFolder(null)}>
                        {t("Back to folders")}
                    </button>
                    <span className="fw-semibold">{openFolder}</span>
                </div>
                {folderImages.length === 0 ? (
                    <div className="text-muted p-3">{t("No images.")}</div>
                ) : (
                    <div style={gridStyle}>{folderImages.map(renderCard)}</div>
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
                <div style={{ ...gridStyle, marginBottom: standalone.length > 0 ? 24 : 0 }}>
                    {folders.map((f) => (
                        <FolderDeck key={f.folder} folder={f} onClick={() => setOpenFolder(f.folder)} />
                    ))}
                </div>
            )}
            {standalone.length > 0 && <div style={gridStyle}>{standalone.map(renderCard)}</div>}
        </div>
    );
};

export default ImagesList;
