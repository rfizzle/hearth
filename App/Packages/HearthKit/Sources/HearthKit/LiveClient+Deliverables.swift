// LiveClient's measure and template calls, in a file of their own to keep
// both short.

import Connect

extension LiveClient {
    public func listMeasures() async throws -> [Hearth_V1_MeasureValue] {
        try await measures.listMeasures(request: Hearth_V1_ListMeasuresRequest()).value().measures
    }

    public func readMeasures(ids: [String]) async throws -> [Hearth_V1_MeasureValue] {
        let request = Hearth_V1_ReadMeasuresRequest.with { $0.ids = ids }
        return try await measures.readMeasures(request: request).value().values
    }

    public func listDeliverableTemplates() async throws -> [Hearth_V1_DeliverableTemplate] {
        try await deliverables.listDeliverableTemplates(
            request: Hearth_V1_ListDeliverableTemplatesRequest()
        ).value().templates
    }

    public func saveDeliverableTemplate(
        json: String
    ) async throws -> Hearth_V1_DeliverableTemplate {
        let request = Hearth_V1_SaveDeliverableTemplateRequest.with { $0.json = json }
        return try await deliverables.saveDeliverableTemplate(request: request).value().template
    }

    public func deleteDeliverableTemplate(id: String) async throws {
        let request = Hearth_V1_DeleteDeliverableTemplateRequest.with { $0.id = id }
        _ = try await deliverables.deleteDeliverableTemplate(request: request).value()
    }
}
