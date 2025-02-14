/*
 * Virtualpaper is a service to manage users paper documents in virtual format.
 * Copyright (C) 2022  Tero Vierimaa
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

import React, { useState } from "react";
import { Link} from 'react-router-dom';
import {useGetManyReference, useRecordContext, Loading} from "react-admin";
import {LimitStringLength} from "../../util";
import {Typography} from "@mui/material";

export const LinkedDocumentList = () => {
    const record = useRecordContext();

    const { data, isLoading, error } = useGetManyReference(
        "documents/linked",
        {
            target: "id",
            id: record?.id,
        }
    );
    if (isLoading) {
        return <Loading/>
    }
    if (data) {
        return (
            <>
                {data.map((doc) => <LinkedDocument name={doc.name} id={doc.id} createdAt={doc.created_at}/>)}
            </>
        )
    }
    return null
}

interface documentProps {
    id: string
    name: string
    createdAt: string
}

const LinkedDocument = (props: documentProps) => {
    const {name, id} = props;
    const limitedName = LimitStringLength(name, 50);
    return (
        <Link to={`/documents/${id}/show`}>
        <Typography variant="body2">{limitedName}</Typography>
        </Link>
    )
}